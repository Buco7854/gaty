package middleware

import (
	"context"
	"net/http"

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

// TenantResolver extracts the tenant from the Host header by looking up
// the custom_domains table. Falls through silently if no match (main domain).
func TenantResolver(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host

			var targetType, targetID string
			err := pool.QueryRow(r.Context(),
				`SELECT target_type, target_id FROM custom_domains
				 WHERE domain_name = $1 AND is_verified = TRUE`,
				host,
			).Scan(&targetType, &targetID)

			if err == nil {
				ctx := context.WithValue(r.Context(), TenantTypeKey, targetType)
				ctx = context.WithValue(ctx, TenantIDKey, targetID)
				r = r.WithContext(ctx)
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
