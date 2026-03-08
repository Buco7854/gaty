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

type WorkspaceHandler struct {
	workspaces *service.WorkspaceService
}

func NewWorkspaceHandler(workspaces *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{workspaces: workspaces}
}

// WorkspacePathParam is shared with gate, member, policy, and schedule handlers.
type WorkspacePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

// --- Create ---

type CreateWorkspaceInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"100" pattern:"^[\\p{L}\\p{N} _\\-\\.]+$"`
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
		return nil, huma.Error500InternalServerError("failed to create workspace", err)
	}
	return &WorkspaceOutput{Body: *ws}, nil
}

// --- List ---

type ListWorkspacesOutput struct {
	Body []model.WorkspaceWithRole
}

func (h *WorkspaceHandler) List(ctx context.Context, _ *struct{}) (*ListWorkspacesOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	list, err := h.workspaces.List(ctx, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list workspaces", err)
	}
	return &ListWorkspacesOutput{Body: list}, nil
}

// --- Get ---

func (h *WorkspaceHandler) Get(ctx context.Context, input *WorkspacePathParam) (*WorkspaceOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	ws, err := h.workspaces.Get(ctx, input.WorkspaceID, userID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if errors.Is(err, model.ErrUnauthorized) {
		return nil, huma.Error403Forbidden("access denied")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get workspace", err)
	}
	return &WorkspaceOutput{Body: *ws}, nil
}

// --- Rename ---

type RenameWorkspaceInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		Name string `json:"name" minLength:"1" maxLength:"100" pattern:"^[\\p{L}\\p{N} _\\-\\.]+$"`
	}
}

func (h *WorkspaceHandler) Rename(ctx context.Context, input *RenameWorkspaceInput) (*WorkspaceOutput, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	ws, err := h.workspaces.Get(ctx, input.WorkspaceID, userID)
	if err != nil || ws.Role != model.RoleOwner {
		return nil, huma.Error403Forbidden("only the workspace owner can rename it")
	}
	renamed, err := h.workspaces.Rename(ctx, input.WorkspaceID, input.Body.Name)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to rename workspace", err)
	}
	return &WorkspaceOutput{Body: model.WorkspaceWithRole{Workspace: *renamed, Role: model.RoleOwner}}, nil
}

// --- Delete ---

func (h *WorkspaceHandler) Delete(ctx context.Context, input *WorkspacePathParam) (*struct{}, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	ws, err := h.workspaces.Get(ctx, input.WorkspaceID, userID)
	if err != nil || ws.Role != model.RoleOwner {
		return nil, huma.Error403Forbidden("only the workspace owner can delete it")
	}
	if err := h.workspaces.Delete(ctx, input.WorkspaceID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, huma.Error404NotFound("workspace not found")
		}
		return nil, huma.Error500InternalServerError("failed to delete workspace", err)
	}
	return nil, nil
}

// --- Member auth config ---

type MemberAuthConfigOutput struct {
	Body map[string]any
}

func (h *WorkspaceHandler) GetMemberAuthConfig(ctx context.Context, input *WorkspacePathParam) (*MemberAuthConfigOutput, error) {
	cfg, err := h.workspaces.GetMemberAuthConfig(ctx, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get member auth config", err)
	}
	return &MemberAuthConfigOutput{Body: cfg}, nil
}

type UpdateMemberAuthConfigInput struct {
	WorkspaceID uuid.UUID      `path:"ws_id"`
	Body        map[string]any `doc:"Member auth configuration"`
}

func (h *WorkspaceHandler) UpdateMemberAuthConfig(ctx context.Context, input *UpdateMemberAuthConfigInput) (*MemberAuthConfigOutput, error) {
	cfg, err := h.workspaces.UpdateMemberAuthConfig(ctx, input.WorkspaceID, input.Body)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update member auth config", err)
	}
	return &MemberAuthConfigOutput{Body: cfg}, nil
}

// --- Routes ---

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
		OperationID: "workspace-rename",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/name",
		Summary:     "Rename a workspace (owner only)",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.Rename)

	huma.Register(api, huma.Operation{
		OperationID:   "workspace-delete",
		Method:        http.MethodDelete,
		Path:          "/api/workspaces/{ws_id}",
		Summary:       "Delete a workspace (owner only)",
		Tags:          []string{"Workspaces"},
		DefaultStatus: http.StatusNoContent,
		Middlewares:   huma.Middlewares{requireAuth},
	}, h.Delete)

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
		Method:      http.MethodPut,
		Path:        "/api/workspaces/{ws_id}/member-auth-config",
		Summary:     "Replace workspace default member auth configuration",
		Description: "Full replacement: send the complete desired config. Use GET first to read the current value.",
		Tags:        []string{"Workspaces"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.UpdateMemberAuthConfig)
}
