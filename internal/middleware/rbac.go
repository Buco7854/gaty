package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GateManager is a Huma per-operation middleware that requires gate:manage permission.
// ADMIN/OWNER are exempt — unless the request is authenticated via an API token, in which case
// the token's explicit gate+permission policies are always enforced regardless of role.
// Must be chained after WorkspaceMember (requires ws_role and ws_membership_id in context).
func GateManager(api huma.API, policyRepo repository.PolicyRepository) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// ADMIN/OWNER bypass only for session-based auth (password/SSO).
		// API tokens are always fine-grained: enforce policy even for privileged roles.
		if !IsAPITokenAuth(ctx.Context()) {
			role, _ := WorkspaceRoleFromContext(ctx.Context())
			if role == model.RoleAdmin || role == model.RoleOwner {
				next(ctx)
				return
			}
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

func workspaceAccess(api huma.API, memberRepo repository.WorkspaceMembershipRepository, minRole model.WorkspaceRole) func(huma.Context, func(huma.Context)) {
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
func WorkspaceMember(api huma.API, memberRepo repository.WorkspaceMembershipRepository) func(huma.Context, func(huma.Context)) {
	return workspaceAccess(api, memberRepo, model.RoleMember)
}

// WorkspaceAdmin is a Huma per-operation middleware that requires OWNER or ADMIN.
// API tokens are rejected regardless of role: tokens are gate-scoped and cannot perform
// workspace management operations (member management, SSO settings, etc.).
func WorkspaceAdmin(api huma.API, memberRepo repository.WorkspaceMembershipRepository) func(huma.Context, func(huma.Context)) {
	inner := workspaceAccess(api, memberRepo, model.RoleAdmin)
	return func(ctx huma.Context, next func(huma.Context)) {
		if IsAPITokenAuth(ctx.Context()) {
			huma.WriteErr(api, ctx, http.StatusForbidden, "API tokens cannot perform workspace management operations")
			return
		}
		inner(ctx, next)
	}
}

// IsPrivilegedMember returns true if the caller is ADMIN or OWNER authenticated via a session
// (password or SSO). Always returns false for API token auth — tokens are fine-grained and
// must never bypass auth config restrictions based on role.
func IsPrivilegedMember(ctx context.Context) bool {
	if IsAPITokenAuth(ctx) {
		return false
	}
	if role, ok := WorkspaceRoleFromContext(ctx); ok {
		return role == model.RoleAdmin || role == model.RoleOwner
	}
	if role, ok := MemberRoleFromContext(ctx); ok {
		return role == model.RoleAdmin || role == model.RoleOwner
	}
	return false
}

// AdminOrGateManager is a Huma per-operation middleware that allows ADMIN/OWNER
// or any member with gate:manage on at least one gate in the workspace.
// Must be chained after WorkspaceMember.
func AdminOrGateManager(api huma.API, memberRepo repository.WorkspaceMembershipRepository, policyRepo repository.PolicyRepository) func(huma.Context, func(huma.Context)) {
	inner := workspaceAccess(api, memberRepo, model.RoleMember)
	return func(ctx huma.Context, next func(huma.Context)) {
		inner(ctx, func(ctx huma.Context) {
			role, _ := WorkspaceRoleFromContext(ctx.Context())
			if role == model.RoleAdmin || role == model.RoleOwner {
				next(ctx)
				return
			}
			membershipID, ok := WorkspaceMembershipIDFromContext(ctx.Context())
			if !ok {
				huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
				return
			}
			wsID, _ := uuid.Parse(chi.URLParamFromCtx(ctx.Context(), "ws_id"))
			isManager, err := policyRepo.HasPermissionInWorkspace(ctx.Context(), membershipID, wsID, "gate:manage")
			if err != nil || !isManager {
				huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
				return
			}
			next(ctx)
		})
	}
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
