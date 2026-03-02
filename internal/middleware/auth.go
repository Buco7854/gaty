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

// AuthExtractor is a global Huma middleware (applied via api.UseMiddleware).
// It silently extracts the Bearer token and injects the user ID into context
// if valid. Always calls next — never rejects on its own.
func AuthExtractor(authSvc *service.AuthService) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if token := bearerToken(ctx); token != "" {
			if userID, err := authSvc.ValidateAccessToken(token); err == nil {
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

// UserIDFromContext returns the authenticated user ID from a handler's
// context.Context (the value set by AuthExtractor via huma.WithValue).
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

func bearerToken(ctx huma.Context) string {
	h := ctx.Header("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
