package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Buco7854/gaty/internal/repository"
	repopg "github.com/Buco7854/gaty/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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

// tenant cache keys and TTLs.
// A negative-result TTL of 30s prevents repeated DB hits for regular API clients
// whose Host header is never a custom domain.
const (
	tenantCachePrefix      = "tenant:domain:"
	tenantCacheTTL         = 5 * time.Minute  // found + verified
	tenantCacheNotFoundTTL = 30 * time.Second // not found or not yet verified
	tenantCacheNotFound    = "\x00"           // sentinel stored for negative results
)

// TenantResolver is a chi middleware that resolves the custom domain from the Host header.
// On a cache hit (Redis), the domain→gate mapping is served without a DB round-trip.
// On a cache miss it queries the DB and caches the result for subsequent requests.
//
// redisClient may be nil — the middleware falls back to DB-only mode.
func TenantResolver(pool *pgxpool.Pool, redisClient *redis.Client) func(http.Handler) http.Handler {
	domainRepo := repopg.NewCustomDomainRepository(pool)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := normalizeHost(r.Host)
			if host == "" {
				next.ServeHTTP(w, r)
				return
			}

			cacheKey := tenantCachePrefix + host

			// 1. Redis cache lookup (avoids DB hit on the hot path).
			if redisClient != nil {
				if val, err := redisClient.Get(r.Context(), cacheKey).Result(); err == nil {
					if val != tenantCacheNotFound {
						r = injectGateTenant(r, val)
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// 2. Cache miss: query DB.
			d, err := domainRepo.GetByDomain(r.Context(), host)
			switch {
			case err == nil && d.IsVerified():
				gateIDStr := d.GateID.String()
				if redisClient != nil {
					_ = redisClient.Set(context.Background(), cacheKey, gateIDStr, tenantCacheTTL).Err()
				}
				r = injectGateTenant(r, gateIDStr)

			case errors.Is(err, repository.ErrNotFound) || (err == nil && !d.IsVerified()):
				// Cache negative result briefly to avoid hammering DB on every normal API request.
				if redisClient != nil {
					_ = redisClient.Set(context.Background(), cacheKey, tenantCacheNotFound, tenantCacheNotFoundTTL).Err()
				}

			default:
				// Unexpected error: log-worthy but not blocking (API still works without tenant).
				_ = err
			}

			next.ServeHTTP(w, r)
		})
	}
}

func injectGateTenant(r *http.Request, gateIDStr string) *http.Request {
	ctx := context.WithValue(r.Context(), TenantTypeKey, TenantTypeGate)
	ctx = context.WithValue(ctx, TenantIDKey, gateIDStr)
	return r.WithContext(ctx)
}

func normalizeHost(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return strings.ToLower(strings.TrimSpace(host))
}

// TenantFromContext returns the resolved tenant type and ID, if any.
func TenantFromContext(ctx context.Context) (tenantType, tenantID string, ok bool) {
	t, tok := ctx.Value(TenantTypeKey).(string)
	id, iok := ctx.Value(TenantIDKey).(string)
	return t, id, tok && iok
}
