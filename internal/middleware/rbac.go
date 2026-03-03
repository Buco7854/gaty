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

const wsRoleKey contextKey = "ws_role"

var roleOrder = map[model.WorkspaceRole]int{
	model.RoleMember: 1,
	model.RoleAdmin:  2,
	model.RoleOwner:  3,
}

func workspaceAccess(api huma.API, wsRepo *repository.WorkspaceRepository, minRole model.WorkspaceRole) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		wsID, err := uuid.Parse(chi.URLParamFromCtx(ctx.Context(), "ws_id"))
		if err != nil {
			huma.WriteErr(api, ctx, http.StatusBadRequest, "invalid workspace id")
			return
		}

		userID, ok := UserIDFromContext(ctx.Context())
		if !ok {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "unauthorized")
			return
		}

		role, err := wsRepo.GetMemberRole(ctx.Context(), wsID, userID)
		if errors.Is(err, repository.ErrNotFound) {
			huma.WriteErr(api, ctx, http.StatusForbidden, "access denied")
			return
		}
		if err != nil {
			huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal error")
			return
		}

		if roleOrder[role] < roleOrder[minRole] {
			huma.WriteErr(api, ctx, http.StatusForbidden, "insufficient permissions")
			return
		}

		ctx = huma.WithValue(ctx, wsRoleKey, role)
		next(ctx)
	}
}

// WorkspaceMember is a Huma per-operation middleware that requires the user to be
// a member (any role) of the workspace in :ws_id. Injects the role into context.
func WorkspaceMember(api huma.API, wsRepo *repository.WorkspaceRepository) func(huma.Context, func(huma.Context)) {
	return workspaceAccess(api, wsRepo, model.RoleMember)
}

// WorkspaceAdmin is a Huma per-operation middleware that requires OWNER or ADMIN.
func WorkspaceAdmin(api huma.API, wsRepo *repository.WorkspaceRepository) func(huma.Context, func(huma.Context)) {
	return workspaceAccess(api, wsRepo, model.RoleAdmin)
}

// WorkspaceRoleFromContext retrieves the workspace role injected by WorkspaceMember/WorkspaceAdmin.
func WorkspaceRoleFromContext(ctx context.Context) (model.WorkspaceRole, bool) {
	role, ok := ctx.Value(wsRoleKey).(model.WorkspaceRole)
	return role, ok && role != ""
}
