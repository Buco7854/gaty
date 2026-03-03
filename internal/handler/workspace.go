package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gaty/internal/middleware"
	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type WorkspaceHandler struct {
	workspaces *repository.WorkspaceRepository
}

func NewWorkspaceHandler(workspaces *repository.WorkspaceRepository) *WorkspaceHandler {
	return &WorkspaceHandler{workspaces: workspaces}
}

// --- Create workspace ---

type CreateWorkspaceInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"100"`
	}
}

type WorkspaceOutput struct {
	Body model.WorkspaceWithRole
}

func (h *WorkspaceHandler) Create(ctx context.Context, input *CreateWorkspaceInput) (*WorkspaceOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	ws, err := h.workspaces.Create(ctx, input.Body.Name, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create workspace")
	}
	return &WorkspaceOutput{Body: model.WorkspaceWithRole{Workspace: *ws, Role: model.RoleOwner}}, nil
}

// --- List workspaces ---

type ListWorkspacesOutput struct {
	Body []model.WorkspaceWithRole
}

func (h *WorkspaceHandler) List(ctx context.Context, _ *struct{}) (*ListWorkspacesOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	list, err := h.workspaces.ListForUser(ctx, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list workspaces")
	}
	if list == nil {
		list = []model.WorkspaceWithRole{}
	}
	return &ListWorkspacesOutput{Body: list}, nil
}

// --- Get workspace ---

type WorkspacePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

func (h *WorkspaceHandler) Get(ctx context.Context, input *WorkspacePathParam) (*WorkspaceOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}

	ws, err := h.workspaces.GetByID(ctx, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get workspace")
	}

	role, err := h.workspaces.GetMemberRole(ctx, ws.ID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error403Forbidden("access denied")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to verify access")
	}

	return &WorkspaceOutput{Body: model.WorkspaceWithRole{Workspace: *ws, Role: role}}, nil
}

// --- Invite member ---

type MemberPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	UserID      uuid.UUID `path:"user_id"`
}

type InviteMemberInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		UserID uuid.UUID           `json:"user_id"`
		Role   model.WorkspaceRole `json:"role"`
	}
}

func (h *WorkspaceHandler) InviteMember(ctx context.Context, input *InviteMemberInput) (*struct{}, error) {
	if input.Body.Role == model.RoleOwner {
		return nil, huma.Error400BadRequest("cannot assign OWNER role via invite")
	}
	err := h.workspaces.AddMember(ctx, input.WorkspaceID, input.Body.UserID, input.Body.Role)
	if errors.Is(err, repository.ErrAlreadyExists) {
		return nil, huma.Error409Conflict("user is already a member")
	}
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("user not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to add member")
	}
	return nil, nil
}

// --- Update member role ---

type UpdateMemberRoleInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	UserID      uuid.UUID `path:"user_id"`
	Body        struct {
		Role model.WorkspaceRole `json:"role"`
	}
}

func (h *WorkspaceHandler) UpdateMemberRole(ctx context.Context, input *UpdateMemberRoleInput) (*struct{}, error) {
	if input.Body.Role == model.RoleOwner {
		return nil, huma.Error400BadRequest("cannot assign OWNER role")
	}
	current, err := h.workspaces.GetMemberRole(ctx, input.WorkspaceID, input.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check member")
	}
	if current == model.RoleOwner {
		return nil, huma.Error403Forbidden("cannot change the workspace owner's role")
	}
	if err := h.workspaces.UpdateMemberRole(ctx, input.WorkspaceID, input.UserID, input.Body.Role); err != nil {
		return nil, huma.Error500InternalServerError("failed to update role")
	}
	return nil, nil
}

// --- Remove member ---

func (h *WorkspaceHandler) RemoveMember(ctx context.Context, input *MemberPathParam) (*struct{}, error) {
	current, err := h.workspaces.GetMemberRole(ctx, input.WorkspaceID, input.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check member")
	}
	if current == model.RoleOwner {
		return nil, huma.Error403Forbidden("cannot remove the workspace owner")
	}
	if err := h.workspaces.RemoveMember(ctx, input.WorkspaceID, input.UserID); err != nil {
		return nil, huma.Error500InternalServerError("failed to remove member")
	}
	return nil, nil
}

// RegisterRoutes wires workspace endpoints onto the Huma API.
func (h *WorkspaceHandler) RegisterRoutes(
	api huma.API,
	requireAuth func(huma.Context, func(huma.Context)),
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID: "workspace-create",
		Method:      http.MethodPost,
		Path:        "/api/workspaces",
		Summary:     "Create a workspace",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID: "workspace-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces",
		Summary:     "List workspaces for current user",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "workspace-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}",
		Summary:     "Get a workspace",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "workspace-invite-user",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/users",
		Summary:     "Add a platform user to a workspace",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.InviteMember)

	huma.Register(api, huma.Operation{
		OperationID: "workspace-update-user-role",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/users/{user_id}",
		Summary:     "Update a workspace user's role",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.UpdateMemberRole)

	huma.Register(api, huma.Operation{
		OperationID: "workspace-remove-user",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/users/{user_id}",
		Summary:     "Remove a platform user from a workspace",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.RemoveMember)
}
