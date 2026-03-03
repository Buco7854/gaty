package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

const userIDKey contextKey = "user_id"
const memberIDKey contextKey = "member_id"
const memberWorkspaceIDKey contextKey = "member_workspace_id"

// AuthExtractor is a global Huma middleware (applied via api.UseMiddleware).
// It silently extracts the Bearer token and injects the identity into context.
// Tries member token first (has explicit "typ":"member" claim), then user token.
// Always calls next — never rejects on its own.
func AuthExtractor(authSvc *service.AuthService) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if token := bearerToken(ctx); token != "" {
			if memberID, wsID, err := authSvc.ValidateMemberToken(token); err == nil {
				ctx = huma.WithValue(ctx, memberIDKey, memberID)
				ctx = huma.WithValue(ctx, memberWorkspaceIDKey, wsID)
			} else if userID, err := authSvc.ValidateAccessToken(token); err == nil {
				ctx = huma.WithValue(ctx, userIDKey, userID)
			}
		}
		next(ctx)
	}
}

// RequireAuth is a Huma per-operation middleware applied via
// huma.Operation{Middlewares: huma.Middlewares{requireAuth}}.
// Returns 401 if no valid user ID is present in context.
// Must run after AuthExtractor (global).
func RequireAuth(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		id, ok := ctx.Context().Value(userIDKey).(uuid.UUID)
		if !ok || id == uuid.Nil {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}
		next(ctx)
	}
}

// RequireMemberAuth is a Huma per-operation middleware that returns 401
// if no valid member token is present in context.
func RequireMemberAuth(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		id, ok := ctx.Context().Value(memberIDKey).(uuid.UUID)
		if !ok || id == uuid.Nil {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}
		next(ctx)
	}
}

// UserIDFromContext returns the authenticated user ID from a handler's context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// MemberFromContext returns the authenticated member ID and workspace ID from context.
func MemberFromContext(ctx context.Context) (memberID, workspaceID uuid.UUID, ok bool) {
	mID, mok := ctx.Value(memberIDKey).(uuid.UUID)
	wsID, wok := ctx.Value(memberWorkspaceIDKey).(uuid.UUID)
	return mID, wsID, mok && wok && mID != uuid.Nil && wsID != uuid.Nil
}

func bearerToken(ctx huma.Context) string {
	h := ctx.Header("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
