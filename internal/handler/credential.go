package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

// CredentialHandler manages credentials for members.
type CredentialHandler struct {
	credRepo       repository.CredentialRepository
	memberRepo     repository.MemberRepository
	credPolicyRepo repository.CredentialPolicyRepository
}

func NewCredentialHandler(
	credRepo repository.CredentialRepository,
	memberRepo repository.MemberRepository,
	credPolicyRepo repository.CredentialPolicyRepository,
) *CredentialHandler {
	return &CredentialHandler{
		credRepo:       credRepo,
		memberRepo:     memberRepo,
		credPolicyRepo: credPolicyRepo,
	}
}

// credentialView is the public DTO for a credential (hashed_value is never exposed).
type credentialView struct {
	ID        uuid.UUID            `json:"id"`
	Type      model.CredentialType `json:"type"`
	Label     *string              `json:"label,omitempty"`
	ExpiresAt *time.Time           `json:"expires_at,omitempty"`
	Metadata  map[string]any       `json:"metadata,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
}

func credView(c *model.Credential) credentialView {
	return credentialView{
		ID: c.ID, Type: c.Type, Label: c.Label,
		ExpiresAt: c.ExpiresAt, Metadata: c.Metadata, CreatedAt: c.CreatedAt,
	}
}

// ─── Shared input/output types ────────────────────────────────────────────────

type credListOutput struct {
	Body []credentialView
}

type policyInput struct {
	GateID         uuid.UUID `json:"gate_id"`
	PermissionCode string    `json:"permission_code" minLength:"1"`
}

type createAPITokenInput struct {
	Body struct {
		Label      string        `json:"label" minLength:"1" maxLength:"100"`
		ExpiresAt  *time.Time    `json:"expires_at,omitempty"`
		Policies   []policyInput `json:"policies,omitempty"`
		ScheduleID *uuid.UUID    `json:"schedule_id,omitempty"`
	}
}

type createAPITokenOutput struct {
	Body struct {
		credentialView
		Token string `json:"token"` // raw token — shown only once at creation
	}
}

type credIDPathParam struct {
	CredID uuid.UUID `path:"cred_id"`
}

type changePasswordInput struct {
	Body struct {
		OldPassword string `json:"old_password" minLength:"1"`
		NewPassword string `json:"new_password" minLength:"8"`
	}
}

// ─── Self-service: /api/auth/me/* ────────────────────────────────────────────

func (h *CredentialHandler) ListMyCredentials(ctx context.Context, input *PaginationQuery) (*credListOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	p := input.Params()
	tokens, _, err := h.credRepo.ListByMemberAndType(ctx, memberID, model.CredAPIToken, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials", err)
	}
	ssos, _, err := h.credRepo.ListByMemberAndType(ctx, memberID, model.CredSSOIdentity, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials", err)
	}

	views := make([]credentialView, 0, len(tokens)+len(ssos))
	for _, c := range tokens {
		views = append(views, credView(c))
	}
	for _, c := range ssos {
		views = append(views, credView(c))
	}
	return &credListOutput{Body: views}, nil
}

func (h *CredentialHandler) CreateMyAPIToken(ctx context.Context, input *createAPITokenInput) (*createAPITokenOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	rawToken, hash, err := service.GenerateAPIToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token", err)
	}

	label := input.Body.Label
	cred, err := h.credRepo.Create(ctx, memberID, model.CredAPIToken, hash, &label, input.Body.ExpiresAt, nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create token", err)
	}

	for _, p := range input.Body.Policies {
		if err := h.credPolicyRepo.Grant(ctx, cred.ID, p.GateID, p.PermissionCode); err != nil {
			return nil, huma.Error500InternalServerError("failed to set token policy", err)
		}
	}
	if input.Body.ScheduleID != nil {
		if err := h.credPolicyRepo.SetSchedule(ctx, cred.ID, *input.Body.ScheduleID); err != nil {
			return nil, huma.Error500InternalServerError("failed to set token schedule", err)
		}
	}

	out := &createAPITokenOutput{}
	out.Body.credentialView = credView(cred)
	out.Body.Token = rawToken
	return out, nil
}

func (h *CredentialHandler) DeleteMyCredential(ctx context.Context, input *credIDPathParam) (*struct{}, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	cred, err := h.credRepo.GetByID(ctx, input.CredID, memberID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("credential not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get credential", err)
	}
	if cred.Type == model.CredPassword {
		return nil, huma.Error400BadRequest("use PATCH /api/auth/me/password to change your password")
	}

	_ = h.credPolicyRepo.RevokeAll(ctx, cred.ID)
	_ = h.credPolicyRepo.RemoveSchedule(ctx, cred.ID)

	if err := h.credRepo.Delete(ctx, input.CredID, memberID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete credential", err)
	}
	return nil, nil
}

func (h *CredentialHandler) ChangeMyPassword(ctx context.Context, input *changePasswordInput) (*struct{}, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	existing, err := h.credRepo.GetByMemberAndType(ctx, memberID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error400BadRequest("no password set on this account")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to retrieve credential", err)
	}

	if err := service.CheckPassword(existing.HashedValue, input.Body.OldPassword); err != nil {
		return nil, huma.Error401Unauthorized("incorrect current password")
	}

	newHash, err := service.HashPassword(input.Body.NewPassword)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash password", err)
	}

	if err := h.credRepo.UpdateHashedValue(ctx, memberID, model.CredPassword, newHash); err != nil {
		return nil, huma.Error500InternalServerError("failed to update password", err)
	}
	return nil, nil
}

// ─── Admin: manage member credentials ────────────────────────────────────────

type memberCredPathParam struct {
	MemberID uuid.UUID `path:"member_id"`
}

type memberCredWithIDPathParam struct {
	MemberID uuid.UUID `path:"member_id"`
	CredID   uuid.UUID `path:"cred_id"`
}

type adminCreateTokenInput struct {
	MemberID uuid.UUID `path:"member_id"`
	Body     struct {
		Label     string     `json:"label" minLength:"1" maxLength:"100"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
}

type adminSetPasswordInput struct {
	MemberID uuid.UUID `path:"member_id"`
	Body     struct {
		Password string `json:"password" minLength:"8"`
	}
}

type adminListMemberCredsInput struct {
	MemberID uuid.UUID `path:"member_id"`
	PaginationQuery
}

func (h *CredentialHandler) validateMember(ctx context.Context, memberID uuid.UUID) error {
	_, err := h.memberRepo.GetByID(ctx, memberID)
	if errors.Is(err, repository.ErrNotFound) {
		return huma.Error404NotFound("member not found")
	}
	if err != nil {
		return huma.Error500InternalServerError("failed to validate member", err)
	}
	return nil
}

func (h *CredentialHandler) AdminListMemberCredentials(ctx context.Context, input *adminListMemberCredsInput) (*credListOutput, error) {
	if err := h.validateMember(ctx, input.MemberID); err != nil {
		return nil, err
	}

	p := input.Params()
	tokens, _, err := h.credRepo.ListByMemberAndType(ctx, input.MemberID, model.CredAPIToken, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials", err)
	}
	ssos, _, err := h.credRepo.ListByMemberAndType(ctx, input.MemberID, model.CredSSOIdentity, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials", err)
	}

	views := make([]credentialView, 0, len(tokens)+len(ssos))
	for _, c := range tokens {
		views = append(views, credView(c))
	}
	for _, c := range ssos {
		views = append(views, credView(c))
	}
	return &credListOutput{Body: views}, nil
}

func (h *CredentialHandler) AdminCreateMemberAPIToken(ctx context.Context, input *adminCreateTokenInput) (*createAPITokenOutput, error) {
	if err := h.validateMember(ctx, input.MemberID); err != nil {
		return nil, err
	}

	rawToken, hash, err := service.GenerateAPIToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token", err)
	}

	label := input.Body.Label
	cred, err := h.credRepo.Create(ctx, input.MemberID, model.CredAPIToken, hash, &label, input.Body.ExpiresAt, nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create token", err)
	}

	out := &createAPITokenOutput{}
	out.Body.credentialView = credView(cred)
	out.Body.Token = rawToken
	return out, nil
}

func (h *CredentialHandler) AdminDeleteMemberCredential(ctx context.Context, input *memberCredWithIDPathParam) (*struct{}, error) {
	if err := h.validateMember(ctx, input.MemberID); err != nil {
		return nil, err
	}

	cred, err := h.credRepo.GetByID(ctx, input.CredID, input.MemberID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("credential not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get credential", err)
	}
	if cred.Type == model.CredPassword {
		return nil, huma.Error400BadRequest("use the set-password endpoint to manage passwords")
	}

	_ = h.credPolicyRepo.RevokeAll(ctx, cred.ID)
	_ = h.credPolicyRepo.RemoveSchedule(ctx, cred.ID)

	if err := h.credRepo.Delete(ctx, input.CredID, input.MemberID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete credential", err)
	}
	return nil, nil
}

func (h *CredentialHandler) AdminSetMemberPassword(ctx context.Context, input *adminSetPasswordInput) (*struct{}, error) {
	if err := h.validateMember(ctx, input.MemberID); err != nil {
		return nil, err
	}

	newHash, err := service.HashPassword(input.Body.Password)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash password", err)
	}

	// Atomic upsert: update if exists, create if not.
	err = h.credRepo.UpdateHashedValue(ctx, input.MemberID, model.CredPassword, newHash)
	if errors.Is(err, repository.ErrNotFound) {
		// No existing password credential — create one.
		if _, err := h.credRepo.Create(ctx, input.MemberID, model.CredPassword, newHash, nil, nil, nil); err != nil {
			return nil, huma.Error500InternalServerError("failed to set password", err)
		}
	} else if err != nil {
		return nil, huma.Error500InternalServerError("failed to set password", err)
	}
	return nil, nil
}

// ─── Route registration ───────────────────────────────────────────────────────

func (h *CredentialHandler) RegisterRoutes(
	api huma.API,
	requireAuth func(huma.Context, func(huma.Context)),
	requireAdmin func(huma.Context, func(huma.Context)),
) {
	// Self-service — own credentials
	huma.Register(api, huma.Operation{
		OperationID: "list-my-credentials",
		Method:      http.MethodGet,
		Path:        "/api/auth/me/credentials",
		Summary:     "List my credentials (API tokens + SSO identities)",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.ListMyCredentials)

	huma.Register(api, huma.Operation{
		OperationID:   "create-my-api-token",
		Method:        http.MethodPost,
		Path:          "/api/auth/me/api-tokens",
		Summary:       "Create an API token (with optional per-gate policies and schedule)",
		Tags:          []string{"Credentials"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{requireAuth},
	}, h.CreateMyAPIToken)

	huma.Register(api, huma.Operation{
		OperationID: "delete-my-credential",
		Method:      http.MethodDelete,
		Path:        "/api/auth/me/credentials/{cred_id}",
		Summary:     "Revoke a credential (API token or SSO identity)",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.DeleteMyCredential)

	huma.Register(api, huma.Operation{
		OperationID: "change-my-password",
		Method:      http.MethodPatch,
		Path:        "/api/auth/me/password",
		Summary:     "Change my password",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.ChangeMyPassword)

	// Admin — manage member credentials
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-member-credentials",
		Method:      http.MethodGet,
		Path:        "/api/members/{member_id}/credentials",
		Summary:     "List a member's credentials",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.AdminListMemberCredentials)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-member-api-token",
		Method:        http.MethodPost,
		Path:          "/api/members/{member_id}/api-tokens",
		Summary:       "Create an API token for a member",
		Tags:          []string{"Credentials"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{requireAdmin},
	}, h.AdminCreateMemberAPIToken)

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-member-credential",
		Method:      http.MethodDelete,
		Path:        "/api/members/{member_id}/credentials/{cred_id}",
		Summary:     "Revoke a member's credential",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.AdminDeleteMemberCredential)

	huma.Register(api, huma.Operation{
		OperationID: "admin-set-member-password",
		Method:      http.MethodPost,
		Path:        "/api/members/{member_id}/password",
		Summary:     "Set or reset a member's password (no old password required)",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.AdminSetMemberPassword)
}
