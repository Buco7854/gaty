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

type PolicyHandler struct {
	policies *service.PolicyService
}

func NewPolicyHandler(policies *service.PolicyService) *PolicyHandler {
	return &PolicyHandler{policies: policies}
}

type ListPoliciesOutput struct {
	Body PaginatedBody[model.MembershipPolicy]
}

// --- List for gate ---

type ListPoliciesInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	PaginationQuery
}

func (h *PolicyHandler) List(ctx context.Context, input *ListPoliciesInput) (*ListPoliciesOutput, error) {
	p := input.Params()
	policies, total, err := h.policies.List(ctx, input.GateID, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list policies", err)
	}
	return &ListPoliciesOutput{Body: NewPaginatedBody(policies, total, p)}, nil
}

// --- List for membership ---

type ListMembershipPoliciesInput struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	MembershipID uuid.UUID `path:"membership_id"`
	PaginationQuery
}

func (h *PolicyHandler) ListByMembership(ctx context.Context, input *ListMembershipPoliciesInput) (*ListPoliciesOutput, error) {
	p := input.Params()
	policies, total, err := h.policies.ListForMembership(ctx, input.MembershipID, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list membership policies", err)
	}
	return &ListPoliciesOutput{Body: NewPaginatedBody(policies, total, p)}, nil
}

// --- List mine (authenticated member's own policies) ---

type ListMyPoliciesInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	PaginationQuery
}

func (h *PolicyHandler) ListMine(ctx context.Context, input *ListMyPoliciesInput) (*ListPoliciesOutput, error) {
	membershipID, ok := middleware.WorkspaceMembershipIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	p := input.Params()
	policies, total, err := h.policies.ListForMembership(ctx, membershipID, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list policies", err)
	}
	return &ListPoliciesOutput{Body: NewPaginatedBody(policies, total, p)}, nil
}

// --- Grant ---

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
		return nil, huma.Error500InternalServerError("failed to grant policy", err)
	}
	return nil, nil
}

// --- Revoke all for a membership ---

type PolicyMembershipPathParam struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	GateID       uuid.UUID `path:"gate_id"`
	MembershipID uuid.UUID `path:"membership_id"`
}

func (h *PolicyHandler) Revoke(ctx context.Context, input *PolicyMembershipPathParam) (*struct{}, error) {
	if err := h.policies.Revoke(ctx, input.MembershipID, input.GateID); err != nil {
		return nil, huma.Error500InternalServerError("failed to revoke policy", err)
	}
	return nil, nil
}

// --- Revoke a specific permission ---

type RevokePermissionPathParam struct {
	WorkspaceID    uuid.UUID `path:"ws_id"`
	GateID         uuid.UUID `path:"gate_id"`
	MembershipID   uuid.UUID `path:"membership_id"`
	PermissionCode string    `path:"permission_code"`
}

func (h *PolicyHandler) RevokePermission(ctx context.Context, input *RevokePermissionPathParam) (*struct{}, error) {
	err := h.policies.RevokePermission(ctx, input.MembershipID, input.GateID, input.PermissionCode)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("policy not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to revoke permission", err)
	}
	return nil, nil
}

// --- Schedule on a member-gate pair ---

type MemberGateScheduleOutput struct {
	Body *model.AccessSchedule
}

func (h *PolicyHandler) GetMemberGateSchedule(ctx context.Context, input *PolicyMembershipPathParam) (*MemberGateScheduleOutput, error) {
	schedule, err := h.policies.GetMemberGateSchedule(ctx, input.MembershipID, input.GateID)
	if errors.Is(err, model.ErrNotFound) {
		return &MemberGateScheduleOutput{Body: nil}, nil
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule", err)
	}
	return &MemberGateScheduleOutput{Body: schedule}, nil
}

type SetMemberGateScheduleInput struct {
	WorkspaceID  uuid.UUID `path:"ws_id"`
	GateID       uuid.UUID `path:"gate_id"`
	MembershipID uuid.UUID `path:"membership_id"`
	Body         struct {
		ScheduleID uuid.UUID `json:"schedule_id"`
	}
}

func (h *PolicyHandler) SetMemberGateSchedule(ctx context.Context, input *SetMemberGateScheduleInput) (*struct{}, error) {
	if err := h.policies.SetMemberGateSchedule(ctx, input.MembershipID, input.GateID, input.Body.ScheduleID); err != nil {
		return nil, huma.Error500InternalServerError("failed to set schedule", err)
	}
	return nil, nil
}

func (h *PolicyHandler) RemoveMemberGateSchedule(ctx context.Context, input *PolicyMembershipPathParam) (*struct{}, error) {
	if err := h.policies.RemoveMemberGateSchedule(ctx, input.MembershipID, input.GateID); err != nil {
		return nil, huma.Error500InternalServerError("failed to remove schedule", err)
	}
	return nil, nil
}

// --- Routes ---

func (h *PolicyHandler) RegisterRoutes(
	api huma.API,
	wsMember func(huma.Context, func(huma.Context)),
	wsAdmin func(huma.Context, func(huma.Context)),
	wsGateManager func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID: "policy-list-mine",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/policies/me",
		Summary:     "List all policies for the authenticated member",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.ListMine)

	huma.Register(api, huma.Operation{
		OperationID: "policy-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies",
		Summary:     "List membership policies for a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "policy-list-by-membership",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members/{membership_id}/policies",
		Summary:     "List all policies for a membership",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.ListByMembership)

	huma.Register(api, huma.Operation{
		OperationID: "policy-grant",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies",
		Summary:     "Grant a permission to a membership on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.Grant)

	huma.Register(api, huma.Operation{
		OperationID: "policy-revoke",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}",
		Summary:     "Revoke all permissions from a membership on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.Revoke)

	huma.Register(api, huma.Operation{
		OperationID: "policy-revoke-permission",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}/{permission_code}",
		Summary:     "Revoke a specific permission from a membership on a gate",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.RevokePermission)

	huma.Register(api, huma.Operation{
		OperationID: "policy-get-schedule",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}/schedule",
		Summary:     "Get the time-restriction schedule for a member-gate pair",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.GetMemberGateSchedule)

	huma.Register(api, huma.Operation{
		OperationID: "policy-set-schedule",
		Method:      http.MethodPut,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}/schedule",
		Summary:     "Attach (or replace) a time-restriction schedule on a member-gate pair",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.SetMemberGateSchedule)

	huma.Register(api, huma.Operation{
		OperationID: "policy-remove-schedule",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/policies/{membership_id}/schedule",
		Summary:     "Remove the time-restriction schedule from a member-gate pair",
		Tags:        []string{"Policies"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.RemoveMemberGateSchedule)
}
