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

// --- Shared path params (used by gate.go and member.go too) ---

type WorkspacePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
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

// --- Get member auth config ---

type MemberAuthConfigOutput struct {
	Body map[string]any
}

func (h *WorkspaceHandler) GetMemberAuthConfig(ctx context.Context, input *WorkspacePathParam) (*MemberAuthConfigOutput, error) {
	ws, err := h.workspaces.GetByID(ctx, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get workspace")
	}
	cfg := ws.MemberAuthConfig
	if cfg == nil {
		cfg = map[string]any{}
	}
	return &MemberAuthConfigOutput{Body: cfg}, nil
}

// --- Update member auth config ---

type UpdateMemberAuthConfigInput struct {
	WorkspaceID uuid.UUID      `path:"ws_id"`
	Body        map[string]any `doc:"Member auth configuration"`
}

func (h *WorkspaceHandler) UpdateMemberAuthConfig(ctx context.Context, input *UpdateMemberAuthConfigInput) (*MemberAuthConfigOutput, error) {
	ws, err := h.workspaces.UpdateMemberAuthConfig(ctx, input.WorkspaceID, input.Body)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update member auth config")
	}
	cfg := ws.MemberAuthConfig
	if cfg == nil {
		cfg = map[string]any{}
	}
	return &MemberAuthConfigOutput{Body: cfg}, nil
}

// RegisterRoutes wires workspace endpoints onto the Huma API.
func (h *WorkspaceHandler) RegisterRoutes(
	api huma.API,
	requireAuth func(huma.Context, func(huma.Context)),
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID:   "workspace-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces",
		Summary:       "Create a workspace",
		Tags:          []string{"Workspaces"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{requireAuth},
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
		OperationID: "workspace-get-member-auth-config",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/member-auth-config",
		Summary:     "Get workspace default member auth configuration",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.GetMemberAuthConfig)

	huma.Register(api, huma.Operation{
		OperationID: "workspace-update-member-auth-config",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/member-auth-config",
		Summary:     "Update workspace default member auth configuration",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.UpdateMemberAuthConfig)
}

