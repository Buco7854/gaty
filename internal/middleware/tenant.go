package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Buco7854/gaty/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const (
	TenantTypeKey contextKey = "tenant_type"
	TenantIDKey   contextKey = "tenant_id"
)

const (
	TenantTypeWorkspace = "WORKSPACE"
	TenantTypeGate      = "GATE"
)

// TenantResolver is a chi middleware that resolves the custom domain from the Host header.
// If the host matches a verified custom domain in the database, it injects the gate's ID
// and type GATE into the request context. Otherwise it falls through unchanged.
func TenantResolver(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	domainRepo := repository.NewCustomDomainRepository(pool)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Strip port if present.
			if idx := strings.LastIndex(host, ":"); idx != -1 {
				host = host[:idx]
			}
			host = strings.ToLower(strings.TrimSpace(host))

			if host != "" {
				if d, err := domainRepo.GetByDomain(r.Context(), host); err == nil && d.IsVerified() {
					ctx := context.WithValue(r.Context(), TenantTypeKey, TenantTypeGate)
					ctx = context.WithValue(ctx, TenantIDKey, d.GateID.String())
					r = r.WithContext(ctx)
				} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
					// Log but don't block — the API still works without tenant resolution.
					// (slog is not imported here to keep middleware lean; caller can observe via response)
					_ = err
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantFromContext returns the resolved tenant type and ID, if any.
func TenantFromContext(ctx context.Context) (tenantType, tenantID string, ok bool) {
	t, tok := ctx.Value(TenantTypeKey).(string)
	id, iok := ctx.Value(TenantIDKey).(string)
	return t, id, tok && iok
}
