package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GateManager is a Huma per-operation middleware that allows ADMIN/OWNER unconditionally,
// and MEMBER only if they have gate:manage permission on the gate in the {gate_id} path param.
// Must be chained after WorkspaceMember (requires ws_role and ws_membership_id in context).
func GateManager(api huma.API, policyRepo *repository.PolicyRepository) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		role, _ := WorkspaceRoleFromContext(ctx.Context())
		if role == model.RoleAdmin || role == model.RoleOwner {
			next(ctx)
			return
		}
		gateID, err := uuid.Parse(chi.URLParamFromCtx(ctx.Context(), "gate_id"))
		if err != nil {
			huma.WriteErr(api, ctx, http.StatusBadRequest, "invalid gate_id")
			return
		}
		membershipID, ok := WorkspaceMembershipIDFromContext(ctx.Context())
		if !ok {
			huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		ok, err = policyRepo.HasPermission(ctx.Context(), membershipID, gateID, "gate:manage")
		if err != nil || !ok {
			huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		next(ctx)
	}
}

const wsRoleKey contextKey = "ws_role"
const wsMembershipIDKey contextKey = "ws_membership_id"

var roleOrder = map[model.WorkspaceRole]int{
	model.RoleMember: 1,
	model.RoleAdmin:  2,
	model.RoleOwner:  3,
}

func workspaceAccess(api huma.API, _ *repository.WorkspaceRepository, memberRepo *repository.WorkspaceMembershipRepository, minRole model.WorkspaceRole) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		wsID, err := uuid.Parse(chi.URLParamFromCtx(ctx.Context(), "ws_id"))
		if err != nil {
			huma.WriteErr(api, ctx, http.StatusBadRequest, "invalid workspace id")
			return
		}

		var role model.WorkspaceRole
		var membershipID uuid.UUID

		if userID, ok := UserIDFromContext(ctx.Context()); ok {
			// Platform user: look up their workspace_memberships row.
			membership, err := memberRepo.GetByUserID(ctx.Context(), wsID, userID)
			if errors.Is(err, repository.ErrNotFound) {
				huma.WriteErr(api, ctx, http.StatusForbidden, "access denied")
				return
			}
			if err != nil {
				huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal error")
				return
			}
			role = membership.Role
			membershipID = membership.ID
		} else if mID, memberWsID, ok := MemberFromContext(ctx.Context()); ok {
			// Managed member: verify workspace matches and look up their membership.
			if memberWsID != wsID {
				huma.WriteErr(api, ctx, http.StatusForbidden, "access denied")
				return
			}
			membership, err := memberRepo.GetByID(ctx.Context(), mID, wsID)
			if errors.Is(err, repository.ErrNotFound) {
				huma.WriteErr(api, ctx, http.StatusForbidden, "access denied")
				return
			}
			if err != nil {
				huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal error")
				return
			}
			role = membership.Role
			membershipID = membership.ID
		} else {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}

		if roleOrder[role] < roleOrder[minRole] {
			huma.WriteErr(api, ctx, http.StatusForbidden, "insufficient permissions")
			return
		}

		ctx = huma.WithValue(ctx, wsRoleKey, role)
		ctx = huma.WithValue(ctx, wsMembershipIDKey, membershipID)
		next(ctx)
	}
}

// WorkspaceMember is a Huma per-operation middleware that requires any role in the workspace.
// Supports both platform users (workspace_memberships.user_id) and managed members (local JWT).
func WorkspaceMember(api huma.API, wsRepo *repository.WorkspaceRepository, memberRepo *repository.WorkspaceMembershipRepository) func(huma.Context, func(huma.Context)) {
	return workspaceAccess(api, wsRepo, memberRepo, model.RoleMember)
}

// WorkspaceAdmin is a Huma per-operation middleware that requires OWNER or ADMIN.
func WorkspaceAdmin(api huma.API, wsRepo *repository.WorkspaceRepository, memberRepo *repository.WorkspaceMembershipRepository) func(huma.Context, func(huma.Context)) {
	return workspaceAccess(api, wsRepo, memberRepo, model.RoleAdmin)
}

// WorkspaceRoleFromContext retrieves the workspace role injected by WorkspaceMember/WorkspaceAdmin.
func WorkspaceRoleFromContext(ctx context.Context) (model.WorkspaceRole, bool) {
	role, ok := ctx.Value(wsRoleKey).(model.WorkspaceRole)
	return role, ok && role != ""
}

// WorkspaceMembershipIDFromContext retrieves the caller's membership_id injected by WorkspaceMember/WorkspaceAdmin.
func WorkspaceMembershipIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(wsMembershipIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
