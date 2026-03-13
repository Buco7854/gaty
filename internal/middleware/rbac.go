package middleware

import (
	"context"
	"net/http"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RequireAdmin is a per-operation middleware that requires the ADMIN role.
// API tokens are rejected: tokens are gate-scoped and cannot perform admin operations.
func RequireAdmin(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if IsAPITokenAuth(ctx.Context()) {
			huma.WriteErr(api, ctx, http.StatusForbidden, "API tokens cannot perform admin operations")
			return
		}
		role, ok := MemberRoleFromContext(ctx.Context())
		if !ok || role != model.RoleAdmin {
			huma.WriteErr(api, ctx, http.StatusForbidden, "admin access required")
			return
		}
		next(ctx)
	}
}

// GateManager is a Huma per-operation middleware that requires gate:manage permission.
// ADMIN members are exempt — unless the request is authenticated via an API token, in which case
// the token's explicit gate+permission policies are always enforced regardless of role.
func GateManager(api huma.API, policyRepo repository.PolicyRepository) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// ADMIN bypass only for session-based auth (password/SSO).
		if !IsAPITokenAuth(ctx.Context()) {
			role, _ := MemberRoleFromContext(ctx.Context())
			if role == model.RoleAdmin {
				next(ctx)
				return
			}
		}
		gateID, err := uuid.Parse(chi.URLParamFromCtx(ctx.Context(), "gate_id"))
		if err != nil {
			huma.WriteErr(api, ctx, http.StatusBadRequest, "invalid gate_id")
			return
		}
		memberID, ok := MemberIDFromContext(ctx.Context())
		if !ok {
			huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		ok, err = policyRepo.HasPermission(ctx.Context(), memberID, gateID, "gate:manage")
		if err != nil || !ok {
			huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		next(ctx)
	}
}

// IsPrivilegedMember returns true if the caller is ADMIN authenticated via a session
// (password or SSO). Always returns false for API token auth — tokens are fine-grained and
// must never bypass restrictions based on role.
func IsPrivilegedMember(ctx context.Context) bool {
	if IsAPITokenAuth(ctx) {
		return false
	}
	role, ok := MemberRoleFromContext(ctx)
	return ok && role == model.RoleAdmin
}

// AdminOrGateManager is a Huma per-operation middleware that allows ADMIN
// or any member with gate:manage on at least one gate.
func AdminOrGateManager(api huma.API, policyRepo repository.PolicyRepository) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		role, _ := MemberRoleFromContext(ctx.Context())
		if role == model.RoleAdmin && !IsAPITokenAuth(ctx.Context()) {
			next(ctx)
			return
		}
		memberID, ok := MemberIDFromContext(ctx.Context())
		if !ok {
			huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		isManager, err := policyRepo.HasPermissionOnAnyGate(ctx.Context(), memberID, "gate:manage")
		if err != nil || !isManager {
			huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		next(ctx)
	}
}
