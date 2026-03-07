package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type MemberHandler struct {
	memberships *service.MembershipService
}

func NewMemberHandler(memberships *service.MembershipService) *MemberHandler {
	return &MemberHandler{memberships: memberships}
}

// --- Shared path params ---

type MemberWorkspacePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

type MemberIDPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	MemberID    uuid.UUID `path:"member_id"`
}

// --- Response body ---

type membershipBody struct {
	ID            uuid.UUID           `json:"id"`
	WorkspaceID   uuid.UUID           `json:"workspace_id"`
	UserID        *uuid.UUID          `json:"user_id,omitempty"`
	UserEmail     *string             `json:"user_email,omitempty"`
	LocalUsername *string             `json:"local_username,omitempty"`
	DisplayName   *string             `json:"display_name,omitempty"`
	Role          model.WorkspaceRole `json:"role"`
	AuthConfig    map[string]any      `json:"auth_config,omitempty"`
}

func toMembershipBody(m *model.WorkspaceMembership) *membershipBody {
	return &membershipBody{
		ID:            m.ID,
		WorkspaceID:   m.WorkspaceID,
		UserID:        m.UserID,
		UserEmail:     m.UserEmail,
		LocalUsername: m.LocalUsername,
		DisplayName:   m.DisplayName,
		Role:          m.Role,
		AuthConfig:    m.AuthConfig,
	}
}

type MemberOutput struct {
	Body *membershipBody
}

type ListMembersOutput struct {
	Body []*membershipBody
}

// --- Create local member ---

type CreateMemberInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		LocalUsername string              `json:"local_username" minLength:"1" maxLength:"50"`
		DisplayName   *string             `json:"display_name,omitempty" maxLength:"100"`
		Password      string              `json:"password" minLength:"8"`
		Role          model.WorkspaceRole `json:"role,omitempty" default:"MEMBER"`
	}
}

func (h *MemberHandler) Create(ctx context.Context, input *CreateMemberInput) (*MemberOutput, error) {
	invitedBy, _ := middleware.UserIDFromContext(ctx)
	role := input.Body.Role
	if role == "" {
		role = model.RoleMember
	}
	membership, err := h.memberships.CreateLocal(ctx,
		input.WorkspaceID,
		input.Body.LocalUsername,
		input.Body.DisplayName,
		input.Body.Password,
		role,
		&invitedBy,
	)
	if errors.Is(err, service.ErrUsernameTaken) {
		return nil, huma.Error409Conflict("username already taken in this workspace")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create member", err)
	}
	return &MemberOutput{Body: toMembershipBody(membership)}, nil
}

// --- Invite platform user ---

type InviteUserInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		UserID      uuid.UUID           `json:"user_id"`
		DisplayName *string             `json:"display_name,omitempty" maxLength:"100"`
		Role        model.WorkspaceRole `json:"role,omitempty" default:"MEMBER"`
	}
}

func (h *MemberHandler) InviteUser(ctx context.Context, input *InviteUserInput) (*MemberOutput, error) {
	inviterID, _ := middleware.UserIDFromContext(ctx)
	role := input.Body.Role
	if role == model.RoleOwner {
		return nil, huma.Error400BadRequest("cannot assign OWNER role via invite")
	}
	if role == "" {
		role = model.RoleMember
	}
	membership, err := h.memberships.InviteUser(ctx,
		input.WorkspaceID,
		input.Body.UserID,
		input.Body.DisplayName,
		role,
		&inviterID,
	)
	if errors.Is(err, service.ErrAlreadyMember) {
		return nil, huma.Error409Conflict("user already has a membership in this workspace")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to invite user", err)
	}
	return &MemberOutput{Body: toMembershipBody(membership)}, nil
}

// --- List ---

func (h *MemberHandler) List(ctx context.Context, input *MemberWorkspacePathParam) (*ListMembersOutput, error) {
	members, err := h.memberships.List(ctx, input.WorkspaceID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list members", err)
	}
	bodies := make([]*membershipBody, len(members))
	for i, m := range members {
		bodies[i] = toMembershipBody(m)
	}
	return &ListMembersOutput{Body: bodies}, nil
}

// --- Get ---

func (h *MemberHandler) Get(ctx context.Context, input *MemberIDPathParam) (*MemberOutput, error) {
	membership, err := h.memberships.GetByID(ctx, input.MemberID, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get member", err)
	}
	return &MemberOutput{Body: toMembershipBody(membership)}, nil
}

// --- Update ---

type UpdateMemberInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	MemberID    uuid.UUID `path:"member_id"`
	Body        struct {
		// Omit a field to leave it unchanged.
		// auth_config: null = reset to NULL (inherit from workspace), omit = unchanged.
		DisplayName   *string                      `json:"display_name,omitempty" maxLength:"100"`
		LocalUsername *string                      `json:"local_username,omitempty" minLength:"1" maxLength:"50"`
		Role          *model.WorkspaceRole         `json:"role,omitempty"`
		AuthConfig    OmittableNullable[map[string]any] `json:"auth_config,omitempty"`
	}
}

func (h *MemberHandler) Update(ctx context.Context, input *UpdateMemberInput) (*MemberOutput, error) {
	if input.Body.Role != nil && *input.Body.Role == model.RoleOwner {
		return nil, huma.Error400BadRequest("cannot assign OWNER role")
	}
	membership, err := h.memberships.Update(ctx, input.MemberID, input.WorkspaceID, service.UpdateMemberParams{
		DisplayName:   input.Body.DisplayName,
		LocalUsername: input.Body.LocalUsername,
		Role:          input.Body.Role,
		AuthConfig:    input.Body.AuthConfig.ToModel(),
	})
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if errors.Is(err, model.ErrAlreadyExists) {
		return nil, huma.Error409Conflict("local_username already taken in this workspace")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update member", err)
	}
	return &MemberOutput{Body: toMembershipBody(membership)}, nil
}

// --- Delete ---

func (h *MemberHandler) Delete(ctx context.Context, input *MemberIDPathParam) (*struct{}, error) {
	err := h.memberships.Delete(ctx, input.MemberID, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete member", err)
	}
	return nil, nil
}

// RegisterRoutes wires member endpoints onto the Huma API.
func (h *MemberHandler) RegisterRoutes(api huma.API, wsAdmin func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID:   "member-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/members",
		Summary:       "Create a managed (local) member",
		Tags:          []string{"Members"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID:   "member-invite-user",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/members/invite",
		Summary:       "Invite a platform user to the workspace",
		Tags:          []string{"Members"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.InviteUser)

	huma.Register(api, huma.Operation{
		OperationID: "member-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members",
		Summary:     "List all memberships",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "member-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members/{member_id}",
		Summary:     "Get a membership",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "member-update",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/members/{member_id}",
		Summary:     "Update a membership (role, display name, auth config)",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Update)

	huma.Register(api, huma.Operation{
		OperationID: "member-delete",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/members/{member_id}",
		Summary:     "Delete a membership",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Delete)
}
