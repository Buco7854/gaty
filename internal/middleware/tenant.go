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

// TenantResolver is a no-op placeholder until custom_domains is re-added to the schema.
func TenantResolver(_ *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
