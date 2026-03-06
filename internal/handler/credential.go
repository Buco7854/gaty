package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// CredentialHandler manages credentials for platform users and managed members.
type CredentialHandler struct {
	credRepo       repository.CredentialRepository
	memberCredRepo repository.MembershipCredentialRepository
	membershipRepo repository.WorkspaceMembershipRepository
	credPolicyRepo repository.CredentialPolicyRepository
}

func NewCredentialHandler(
	credRepo repository.CredentialRepository,
	memberCredRepo repository.MembershipCredentialRepository,
	membershipRepo repository.WorkspaceMembershipRepository,
	credPolicyRepo repository.CredentialPolicyRepository,
) *CredentialHandler {
	return &CredentialHandler{
		credRepo:       credRepo,
		memberCredRepo: memberCredRepo,
		membershipRepo: membershipRepo,
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

func memberCredView(c *model.MembershipCredential) credentialView {
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

// ─── Platform user: own credentials ──────────────────────────────────────────

func (h *CredentialHandler) ListMyCredentials(ctx context.Context, _ *struct{}) (*credListOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	tokens, err := h.credRepo.ListByUserAndType(ctx, userID, model.CredAPIToken)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
	}
	ssos, err := h.credRepo.ListByUserAndType(ctx, userID, model.CredSSOIdentity)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
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

func (h *CredentialHandler) DeleteMyCredential(ctx context.Context, input *credIDPathParam) (*struct{}, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	cred, err := h.credRepo.GetByID(ctx, input.CredID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("credential not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get credential")
	}
	if cred.UserID != userID {
		return nil, huma.Error404NotFound("credential not found")
	}
	if cred.Type == model.CredPassword {
		return nil, huma.Error400BadRequest("use PATCH /api/auth/me/password to change your password")
	}

	if err := h.credRepo.Delete(ctx, input.CredID, userID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete credential")
	}
	return nil, nil
}

func (h *CredentialHandler) ChangeMyPassword(ctx context.Context, input *changePasswordInput) (*struct{}, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}

	existing, err := h.credRepo.GetByUserAndType(ctx, userID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error400BadRequest("no password set on this account")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to retrieve credential")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(existing.HashedValue), []byte(input.Body.OldPassword)); err != nil {
		return nil, huma.Error401Unauthorized("incorrect current password")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash password")
	}

	if err := h.credRepo.Delete(ctx, existing.ID, userID); err != nil {
		return nil, huma.Error500InternalServerError("failed to update password")
	}
	if _, err := h.credRepo.Create(ctx, userID, model.CredPassword, string(newHash), nil, nil, nil); err != nil {
		return nil, huma.Error500InternalServerError("failed to update password")
	}
	return nil, nil
}

// ─── Workspace member self-service: /api/workspaces/{ws_id}/members/me/* ─────

type workspaceSelfCredPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

type workspaceSelfCredWithIDPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	CredID      uuid.UUID `path:"cred_id"`
}

type createWorkspaceMemberTokenInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		Label      string        `json:"label" minLength:"1" maxLength:"100"`
		ExpiresAt  *time.Time    `json:"expires_at,omitempty"`
		Policies   []policyInput `json:"policies,omitempty"`
		ScheduleID *uuid.UUID    `json:"schedule_id,omitempty"`
	}
}

func (h *CredentialHandler) ListMyWorkspaceMemberCredentials(ctx context.Context, _ *workspaceSelfCredPathParam) (*credListOutput, error) {
	membershipID, ok := middleware.WorkspaceMembershipIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated as workspace member")
	}

	tokens, err := h.memberCredRepo.ListByMembershipAndType(ctx, membershipID, model.CredAPIToken)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
	}

	views := make([]credentialView, 0, len(tokens))
	for _, c := range tokens {
		views = append(views, memberCredView(c))
	}
	return &credListOutput{Body: views}, nil
}

func (h *CredentialHandler) CreateMyWorkspaceMemberAPIToken(ctx context.Context, input *createWorkspaceMemberTokenInput) (*createAPITokenOutput, error) {
	membershipID, ok := middleware.WorkspaceMembershipIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated as workspace member")
	}

	rawToken, hash, err := generateAPIToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	label := input.Body.Label
	cred, err := h.memberCredRepo.Create(ctx, membershipID, model.CredAPIToken, hash, &label, input.Body.ExpiresAt, nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create token")
	}

	for _, p := range input.Body.Policies {
		if err := h.credPolicyRepo.Grant(ctx, cred.ID, p.GateID, p.PermissionCode); err != nil {
			return nil, huma.Error500InternalServerError("failed to set token policy")
		}
	}
	if input.Body.ScheduleID != nil {
		if err := h.credPolicyRepo.SetSchedule(ctx, cred.ID, *input.Body.ScheduleID); err != nil {
			return nil, huma.Error500InternalServerError("failed to set token schedule")
		}
	}

	out := &createAPITokenOutput{}
	out.Body.credentialView = memberCredView(cred)
	out.Body.Token = rawToken
	return out, nil
}

func (h *CredentialHandler) DeleteMyWorkspaceMemberCredential(ctx context.Context, input *workspaceSelfCredWithIDPathParam) (*struct{}, error) {
	membershipID, ok := middleware.WorkspaceMembershipIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated as workspace member")
	}

	cred, err := h.memberCredRepo.GetByID(ctx, input.CredID, membershipID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("credential not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get credential")
	}
	if cred.Type == model.CredPassword {
		return nil, huma.Error400BadRequest("cannot delete a password credential")
	}

	_ = h.credPolicyRepo.RevokeAll(ctx, cred.ID)
	_ = h.credPolicyRepo.RemoveSchedule(ctx, cred.ID)

	if err := h.memberCredRepo.Delete(ctx, input.CredID, membershipID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete credential")
	}
	return nil, nil
}

// ─── Local member: own credentials ───────────────────────────────────────────

// requireLocalMember extracts the local membership ID from context.
// Returns 403 if the request is authenticated as a platform user (not a local member).
func requireLocalMember(ctx context.Context) (uuid.UUID, error) {
	membershipID, _, ok := middleware.MemberFromContext(ctx)
	if !ok {
		return uuid.Nil, huma.Error403Forbidden("this endpoint requires local membership authentication")
	}
	return membershipID, nil
}

func (h *CredentialHandler) ListMyMemberCredentials(ctx context.Context, _ *struct{}) (*credListOutput, error) {
	membershipID, err := requireLocalMember(ctx)
	if err != nil {
		return nil, err
	}

	tokens, err := h.memberCredRepo.ListByMembershipAndType(ctx, membershipID, model.CredAPIToken)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
	}
	ssos, err := h.memberCredRepo.ListByMembershipAndType(ctx, membershipID, model.CredSSOIdentity)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
	}

	views := make([]credentialView, 0, len(tokens)+len(ssos))
	for _, c := range tokens {
		views = append(views, memberCredView(c))
	}
	for _, c := range ssos {
		views = append(views, memberCredView(c))
	}
	return &credListOutput{Body: views}, nil
}

func (h *CredentialHandler) CreateMyMemberAPIToken(ctx context.Context, input *createAPITokenInput) (*createAPITokenOutput, error) {
	membershipID, err := requireLocalMember(ctx)
	if err != nil {
		return nil, err
	}

	rawToken, hash, err := generateAPIToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	label := input.Body.Label
	cred, err := h.memberCredRepo.Create(ctx, membershipID, model.CredAPIToken, hash, &label, input.Body.ExpiresAt, nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create token")
	}

	for _, p := range input.Body.Policies {
		if err := h.credPolicyRepo.Grant(ctx, cred.ID, p.GateID, p.PermissionCode); err != nil {
			return nil, huma.Error500InternalServerError("failed to set token policy")
		}
	}
	if input.Body.ScheduleID != nil {
		if err := h.credPolicyRepo.SetSchedule(ctx, cred.ID, *input.Body.ScheduleID); err != nil {
			return nil, huma.Error500InternalServerError("failed to set token schedule")
		}
	}

	out := &createAPITokenOutput{}
	out.Body.credentialView = memberCredView(cred)
	out.Body.Token = rawToken
	return out, nil
}

func (h *CredentialHandler) DeleteMyMemberCredential(ctx context.Context, input *credIDPathParam) (*struct{}, error) {
	membershipID, err := requireLocalMember(ctx)
	if err != nil {
		return nil, err
	}

	cred, err := h.memberCredRepo.GetByID(ctx, input.CredID, membershipID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("credential not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get credential")
	}
	if cred.Type == model.CredPassword {
		return nil, huma.Error400BadRequest("use PATCH /api/auth/local/me/password to change your password")
	}

	_ = h.credPolicyRepo.RevokeAll(ctx, cred.ID)
	_ = h.credPolicyRepo.RemoveSchedule(ctx, cred.ID)

	if err := h.memberCredRepo.Delete(ctx, input.CredID, membershipID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete credential")
	}
	return nil, nil
}

func (h *CredentialHandler) ChangeMyMemberPassword(ctx context.Context, input *changePasswordInput) (*struct{}, error) {
	membershipID, err := requireLocalMember(ctx)
	if err != nil {
		return nil, err
	}

	existing, err := h.memberCredRepo.GetByMembershipAndType(ctx, membershipID, model.CredPassword)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error400BadRequest("no password set on this membership")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to retrieve credential")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(existing.HashedValue), []byte(input.Body.OldPassword)); err != nil {
		return nil, huma.Error401Unauthorized("incorrect current password")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash password")
	}

	if err := h.memberCredRepo.Delete(ctx, existing.ID, membershipID); err != nil {
		return nil, huma.Error500InternalServerError("failed to update password")
	}
	if _, err := h.memberCredRepo.Create(ctx, membershipID, model.CredPassword, string(newHash), nil, nil, nil); err != nil {
		return nil, huma.Error500InternalServerError("failed to update password")
	}
	return nil, nil
}

// ─── Admin: manage member credentials ────────────────────────────────────────

type memberCredPathParam struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	MembershipID uuid.UUID `path:"membership_id"`
}

type memberCredWithIDPathParam struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	MembershipID uuid.UUID `path:"membership_id"`
	CredID       uuid.UUID `path:"cred_id"`
}

type adminCreateTokenInput struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	MembershipID uuid.UUID `path:"membership_id"`
	Body         struct {
		Label     string     `json:"label" minLength:"1" maxLength:"100"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
}

type adminSetPasswordInput struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	MembershipID uuid.UUID `path:"membership_id"`
	Body         struct {
		Password string `json:"password" minLength:"8"`
	}
}

// validateMembership ensures the membership belongs to the workspace (prevents cross-workspace access).
func (h *CredentialHandler) validateMembership(ctx context.Context, membershipID, workspaceID uuid.UUID) error {
	_, err := h.membershipRepo.GetByID(ctx, membershipID, workspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return huma.Error404NotFound("membership not found in workspace")
	}
	if err != nil {
		return huma.Error500InternalServerError("failed to validate membership")
	}
	return nil
}

func (h *CredentialHandler) AdminListMemberCredentials(ctx context.Context, input *memberCredPathParam) (*credListOutput, error) {
	if err := h.validateMembership(ctx, input.MembershipID, input.WorkspaceID); err != nil {
		return nil, err
	}

	tokens, err := h.memberCredRepo.ListByMembershipAndType(ctx, input.MembershipID, model.CredAPIToken)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
	}
	ssos, err := h.memberCredRepo.ListByMembershipAndType(ctx, input.MembershipID, model.CredSSOIdentity)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list credentials")
	}

	views := make([]credentialView, 0, len(tokens)+len(ssos))
	for _, c := range tokens {
		views = append(views, memberCredView(c))
	}
	for _, c := range ssos {
		views = append(views, memberCredView(c))
	}
	return &credListOutput{Body: views}, nil
}

func (h *CredentialHandler) AdminCreateMemberAPIToken(ctx context.Context, input *adminCreateTokenInput) (*createAPITokenOutput, error) {
	if err := h.validateMembership(ctx, input.MembershipID, input.WorkspaceID); err != nil {
		return nil, err
	}

	rawToken, hash, err := generateAPIToken()
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	label := input.Body.Label
	cred, err := h.memberCredRepo.Create(ctx, input.MembershipID, model.CredAPIToken, hash, &label, input.Body.ExpiresAt, nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create token")
	}

	out := &createAPITokenOutput{}
	out.Body.credentialView = memberCredView(cred)
	out.Body.Token = rawToken
	return out, nil
}

func (h *CredentialHandler) AdminDeleteMemberCredential(ctx context.Context, input *memberCredWithIDPathParam) (*struct{}, error) {
	if err := h.validateMembership(ctx, input.MembershipID, input.WorkspaceID); err != nil {
		return nil, err
	}

	cred, err := h.memberCredRepo.GetByID(ctx, input.CredID, input.MembershipID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("credential not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get credential")
	}
	if cred.Type == model.CredPassword {
		return nil, huma.Error400BadRequest("use the set-password endpoint to manage passwords")
	}

	_ = h.credPolicyRepo.RevokeAll(ctx, cred.ID)
	_ = h.credPolicyRepo.RemoveSchedule(ctx, cred.ID)

	if err := h.memberCredRepo.Delete(ctx, input.CredID, input.MembershipID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete credential")
	}
	return nil, nil
}

func (h *CredentialHandler) AdminSetMemberPassword(ctx context.Context, input *adminSetPasswordInput) (*struct{}, error) {
	if err := h.validateMembership(ctx, input.MembershipID, input.WorkspaceID); err != nil {
		return nil, err
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash password")
	}

	// Replace existing password if any (upsert via delete+create).
	if existing, err := h.memberCredRepo.GetByMembershipAndType(ctx, input.MembershipID, model.CredPassword); err == nil {
		_ = h.memberCredRepo.Delete(ctx, existing.ID, input.MembershipID)
	}

	if _, err := h.memberCredRepo.Create(ctx, input.MembershipID, model.CredPassword, string(newHash), nil, nil, nil); err != nil {
		return nil, huma.Error500InternalServerError("failed to set password")
	}
	return nil, nil
}

// ─── Route registration ───────────────────────────────────────────────────────

func (h *CredentialHandler) RegisterRoutes(
	api huma.API,
	requireAuth func(huma.Context, func(huma.Context)),
	requireMembership func(huma.Context, func(huma.Context)),
	wsMember func(huma.Context, func(huma.Context)),
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	// Platform user — own credentials (SSO identities, password)
	huma.Register(api, huma.Operation{
		OperationID: "list-my-credentials",
		Method:      http.MethodGet,
		Path:        "/api/auth/me/credentials",
		Summary:     "List my credentials (SSO identities)",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.ListMyCredentials)

	huma.Register(api, huma.Operation{
		OperationID: "delete-my-credential",
		Method:      http.MethodDelete,
		Path:        "/api/auth/me/credentials/{cred_id}",
		Summary:     "Revoke a credential (SSO identity)",
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

	// Workspace member self-service — own workspace membership API tokens
	huma.Register(api, huma.Operation{
		OperationID: "list-my-workspace-member-credentials",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members/me/credentials",
		Summary:     "List my workspace membership API tokens",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.ListMyWorkspaceMemberCredentials)

	huma.Register(api, huma.Operation{
		OperationID:   "create-my-workspace-member-api-token",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/members/me/api-tokens",
		Summary:       "Create a workspace membership API token",
		Tags:          []string{"Credentials"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{wsMember},
	}, h.CreateMyWorkspaceMemberAPIToken)

	huma.Register(api, huma.Operation{
		OperationID: "delete-my-workspace-member-credential",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/members/me/credentials/{cred_id}",
		Summary:     "Revoke a workspace membership credential",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.DeleteMyWorkspaceMemberCredential)

	// Local member — own credentials
	huma.Register(api, huma.Operation{
		OperationID: "list-my-member-credentials",
		Method:      http.MethodGet,
		Path:        "/api/auth/local/me/credentials",
		Summary:     "List my membership credentials (API tokens + SSO identities)",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireMembership},
	}, h.ListMyMemberCredentials)

	huma.Register(api, huma.Operation{
		OperationID:   "create-my-member-api-token",
		Method:        http.MethodPost,
		Path:          "/api/auth/local/me/api-tokens",
		Summary:       "Create a membership API token (with optional per-gate policies and schedule)",
		Tags:          []string{"Credentials"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{requireMembership},
	}, h.CreateMyMemberAPIToken)

	huma.Register(api, huma.Operation{
		OperationID: "delete-my-member-credential",
		Method:      http.MethodDelete,
		Path:        "/api/auth/local/me/credentials/{cred_id}",
		Summary:     "Revoke a membership credential",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireMembership},
	}, h.DeleteMyMemberCredential)

	huma.Register(api, huma.Operation{
		OperationID: "change-my-member-password",
		Method:      http.MethodPatch,
		Path:        "/api/auth/local/me/password",
		Summary:     "Change my membership password",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{requireMembership},
	}, h.ChangeMyMemberPassword)

	// Admin — manage member credentials
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-member-credentials",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members/{membership_id}/credentials",
		Summary:     "List a member's credentials",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.AdminListMemberCredentials)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-member-api-token",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/members/{membership_id}/api-tokens",
		Summary:       "Create an API token for a member",
		Tags:          []string{"Credentials"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.AdminCreateMemberAPIToken)

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-member-credential",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/members/{membership_id}/credentials/{cred_id}",
		Summary:     "Revoke a member's credential",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.AdminDeleteMemberCredential)

	huma.Register(api, huma.Operation{
		OperationID: "admin-set-member-password",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/members/{membership_id}/password",
		Summary:     "Set or reset a member's password (no old password required)",
		Tags:        []string{"Credentials"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.AdminSetMemberPassword)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// generateAPIToken creates a new API token.
// Returns the raw token (shown once to the user) and its SHA-256 hash (stored in DB).
// At auth time: SHA256(presented_token) is compared against the stored hash.
func generateAPIToken() (rawToken, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	rawToken = "gatie_" + hex.EncodeToString(b)
	h256 := sha256.Sum256([]byte(rawToken))
	hash = hex.EncodeToString(h256[:])
	return
}
