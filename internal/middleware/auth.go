package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

const memberIDKey contextKey = "member_id"
const memberRoleKey contextKey = "member_role"
const clientIPKey contextKey = "client_ip"
const credentialIDKey contextKey = "credential_id"

// ClientIPInjector is a chi middleware that stores the real client IP in context.
// Must run after chimw.RealIP so that r.RemoteAddr already reflects the real IP.
func ClientIPInjector() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey, ip)))
		})
	}
}

// ClientIPFromContext returns the client IP injected by ClientIPInjector.
func ClientIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(clientIPKey).(string); ok && ip != "" {
		return ip
	}
	return "unknown"
}

// AuthExtractor is a global Huma middleware (applied via api.UseMiddleware).
// It silently extracts authentication and injects the identity into context.
// Priority: 1) gatie_access cookie (JWT), 2) Authorization: Bearer with gatie_* API token.
// Always calls next — never rejects on its own.
func AuthExtractor(authSvc *service.AuthService, credRepo repository.CredentialRepository) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		// 1. Try the HttpOnly access cookie (contains JWT).
		if token := cookieValue(ctx, "gatie_access"); token != "" {
			ctx = extractJWT(ctx, authSvc, token)
		} else if token := bearerToken(ctx); token != "" && strings.HasPrefix(token, "gatie_") {
			// 2. Fallback: API tokens only (gatie_* prefix, looked up via SHA-256 hash).
			h := sha256.Sum256([]byte(token))
			hash := hex.EncodeToString(h[:])
			if cred, member, err := credRepo.FindByHashedAPIToken(ctx.Context(), hash); err == nil {
				if model.APITokenEnabled(member.AuthConfig) {
					ctx = huma.WithValue(ctx, memberIDKey, member.ID)
					ctx = huma.WithValue(ctx, memberRoleKey, member.Role)
					ctx = huma.WithValue(ctx, credentialIDKey, cred.ID)
				}
			}
		}
		next(ctx)
	}
}

// extractJWT validates a JWT and injects member identity into context.
func extractJWT(ctx huma.Context, authSvc *service.AuthService, token string) huma.Context {
	if memberID, role, err := authSvc.ValidateAccessToken(token); err == nil {
		ctx = huma.WithValue(ctx, memberIDKey, memberID)
		ctx = huma.WithValue(ctx, memberRoleKey, role)
	}
	return ctx
}

// cookieValue reads a named cookie from the request.
func cookieValue(ctx huma.Context, name string) string {
	header := ctx.Header("Cookie")
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if eq := strings.IndexByte(part, '='); eq > 0 {
			if part[:eq] == name {
				return part[eq+1:]
			}
		}
	}
	return ""
}

// RequireAuth is a per-operation middleware that requires a valid member JWT or API token.
// Returns 401 if the request is not authenticated.
func RequireAuth(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if id, ok := ctx.Context().Value(memberIDKey).(uuid.UUID); ok && id != uuid.Nil {
			next(ctx)
			return
		}
		huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
	}
}

// MemberIDFromContext returns the authenticated member ID from context.
func MemberIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(memberIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// MemberRoleFromContext returns the role stored in the JWT claims.
func MemberRoleFromContext(ctx context.Context) (model.Role, bool) {
	role, ok := ctx.Value(memberRoleKey).(model.Role)
	return role, ok && role != ""
}

// CredentialIDFromContext returns the API credential ID if the request was authenticated via an API token.
func CredentialIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(credentialIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// IsAPITokenAuth reports whether the current request was authenticated via an API token (gatie_* prefix).
// API tokens are always fine-grained: restricted to explicit gate+permission policies regardless of role.
func IsAPITokenAuth(ctx context.Context) bool {
	_, ok := CredentialIDFromContext(ctx)
	return ok
}

func bearerToken(ctx huma.Context) string {
	h := ctx.Header("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
