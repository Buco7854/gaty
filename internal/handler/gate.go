package handler

import (
	"context"
	"net/http"

	"github.com/Buco7854/gaty/internal/middleware"
	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
)

type GateHandler struct {
	gates *repository.GateRepository
}

func NewGateHandler(gates *repository.GateRepository) *GateHandler {
	return &GateHandler{gates: gates}
}

// --- List gates ---

type ListGatesOutput struct {
	Body []model.Gate
}

func (h *GateHandler) List(ctx context.Context, input *WorkspacePathParam) (*ListGatesOutput, error) {
	userID, _ := middleware.UserIDFromContext(ctx)
	role, _ := middleware.WorkspaceRoleFromContext(ctx)

	gates, err := h.gates.ListForWorkspace(ctx, input.WorkspaceID, userID, role)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list gates")
	}
	if gates == nil {
		gates = []model.Gate{}
	}
	return &ListGatesOutput{Body: gates}, nil
}

// RegisterRoutes wires gate endpoints onto the Huma API.
// wsMember is a Huma per-operation middleware from middleware.WorkspaceMember(api, wsRepo).
func (h *GateHandler) RegisterRoutes(api huma.API, wsMember func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID: "gate-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates",
		Summary:     "List gates for a workspace",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.List)
}
