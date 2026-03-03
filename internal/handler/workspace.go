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
		Slug string `json:"slug" minLength:"1" maxLength:"50" pattern:"^[a-z0-9-]+$" doc:"URL-safe slug, lowercase letters, numbers and hyphens only"`
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
	ws, err := h.workspaces.Create(ctx, input.Body.Slug, input.Body.Name, userID)
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

// RegisterRoutes wires workspace endpoints onto the Huma API.
func (h *WorkspaceHandler) RegisterRoutes(
	api huma.API,
	requireAuth func(huma.Context, func(huma.Context)),
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
}

