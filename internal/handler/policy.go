package handler

import (
	"context"
	"errors"
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

type PolicyMembershipPathParam struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	GateID       uuid.UUID `path:"gate_id"`
	MembershipID uuid.UUID `path:"membership_id"`
}

// --- List policies for a gate ---

type ListPoliciesOutput struct {
	Body []model.MembershipPolicy
}

func (h *PolicyHandler) List(ctx context.Context, input *GatePathParam) (*ListPoliciesOutput, error) {
	policies, err := h.policies.List(ctx, input.GateID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list policies")
	}
	if policies == nil {
		policies = []model.MembershipPolicy{}
	}
	return &ListPoliciesOutput{Body: policies}, nil
}

// --- Grant policy ---

type GrantPolicyInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		MembershipID   uuid.UUID `json:"membership_id"`
		PermissionCode string    `json:"permission_code" minLength:"1"`
	}
}

func (h *PolicyHandler) Grant(ctx context.Context, input *GrantPolicyInput) (*struct{}, error) {
	if err := h.policies.Grant(ctx, input.Body.MembershipID, input.GateID, input.Body.PermissionCode); err != nil {
		return nil, huma.Error500InternalServerError("failed to grant policy")
	}
	return nil, nil
}

// --- Revoke all policies for a membership on a gate ---

func (h *PolicyHandler) Revoke(ctx context.Context, input *PolicyMembershipPathParam) (*struct{}, error) {
	if err := h.policies.Revoke(ctx, input.MembershipID, input.GateID); err != nil {
		return nil, huma.Error500InternalServerError("failed to revoke policy")
	}
	return nil, nil
}

// --- Revoke a single permission for a membership on a gate ---

type RevokePermissionPathParam struct {
	WorkspaceID    uuid.UUID `path:"ws_id"`
	GateID         uuid.UUID `path:"gate_id"`
	MembershipID   uuid.UUID `path:"membership_id"`
	PermissionCode string    `path:"permission_code"`
}

func (h *PolicyHandler) RevokePermission(ctx context.Context, input *RevokePermissionPathParam) (*struct{}, error) {
	err := h.policies.RevokePermission(ctx, input.MembershipID, input.GateID, input.PermissionCode)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("policy not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to revoke permission")
	}
	return nil, nil
}

// RegisterRoutes wires policy endpoints onto the Huma API.
func (h *PolicyHandler) RegisterRoutes(api huma.API, wsAdmin func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID: "policy-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies",
		Summary:     "List membership policies for a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "policy-grant",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies",
		Summary:     "Grant a permission to a membership on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Grant)

	huma.Register(api, huma.Operation{
		OperationID: "policy-revoke",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}",
		Summary:     "Revoke all permissions from a membership on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Revoke)

	huma.Register(api, huma.Operation{
		OperationID: "policy-revoke-permission",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}/{permission_code}",
		Summary:     "Revoke a specific permission from a membership on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.RevokePermission)
}
