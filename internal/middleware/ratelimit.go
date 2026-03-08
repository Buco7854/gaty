package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
)

// RateLimiter returns a Huma middleware that enforces per-IP rate limiting via Redis.
// If Redis is unavailable the request is denied (fail-closed).
func RateLimiter(api huma.API, redisClient *redis.Client, prefix string, maxAttempts int64, window time.Duration) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ip := ctx.Context().Value(clientIPKey)
		if ip == nil {
			ip = ctx.RemoteAddr()
		}
		key := fmt.Sprintf("rl:%s:%v", prefix, ip)

		pipe := redisClient.TxPipeline()
		incrCmd := pipe.Incr(ctx.Context(), key)
		pipe.ExpireNX(ctx.Context(), key, window)
		if _, err := pipe.Exec(ctx.Context()); err != nil {
			// Fail-closed: deny if Redis is unavailable.
			huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "service temporarily unavailable")
			return
		}

		if incrCmd.Val() > maxAttempts {
			ctx.SetHeader("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			huma.WriteErr(api, ctx, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}

		next(ctx)
	}
}
