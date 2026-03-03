package handler

import (
	"context"
	"net/http"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type PolicyHandler struct {
	policies *repository.PolicyRepository
}

func NewPolicyHandler(policies *repository.PolicyRepository) *PolicyHandler {
	return &PolicyHandler{policies: policies}
}

// --- Path param types ---

type GatePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

type PolicyUserPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	UserID      uuid.UUID `path:"user_id"`
}

// --- List policies ---

type ListPoliciesOutput struct {
	Body []model.GatePolicy
}

func (h *PolicyHandler) List(ctx context.Context, input *GatePathParam) (*ListPoliciesOutput, error) {
	policies, err := h.policies.List(ctx, input.GateID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list policies")
	}
	if policies == nil {
		policies = []model.GatePolicy{}
	}
	return &ListPoliciesOutput{Body: policies}, nil
}

// --- Grant policy ---

type GrantPolicyInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		UserID         uuid.UUID `json:"user_id"`
		PermissionCode string    `json:"permission_code"`
	}
}

func (h *PolicyHandler) Grant(ctx context.Context, input *GrantPolicyInput) (*struct{}, error) {
	if err := h.policies.Grant(ctx, input.GateID, input.Body.UserID, input.Body.PermissionCode); err != nil {
		return nil, huma.Error500InternalServerError("failed to grant policy")
	}
	return nil, nil
}

// --- Revoke policy ---

func (h *PolicyHandler) Revoke(ctx context.Context, input *PolicyUserPathParam) (*struct{}, error) {
	if err := h.policies.Revoke(ctx, input.GateID, input.UserID); err != nil {
		return nil, huma.Error500InternalServerError("failed to revoke policy")
	}
	return nil, nil
}

// RegisterRoutes wires policy endpoints onto the Huma API.
// wsAdmin is a Huma per-operation middleware from middleware.WorkspaceAdmin(api, wsRepo).
func (h *PolicyHandler) RegisterRoutes(api huma.API, wsAdmin func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID: "policy-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies",
		Summary:     "List policies for a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "policy-grant",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies",
		Summary:     "Grant a permission to a user on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Grant)

	huma.Register(api, huma.Operation{
		OperationID: "policy-revoke",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{user_id}",
		Summary:     "Revoke all permissions from a user on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Revoke)
}
