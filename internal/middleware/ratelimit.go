package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
)

// RateLimiter returns a Huma middleware that enforces per-IP rate limiting
// using Redis. If the limit is exceeded, a 429 Too Many Requests is returned.
// When Redis is unavailable, the request is allowed (fail-open) to avoid
// blocking legitimate traffic due to infrastructure issues.
func RateLimiter(api huma.API, redisClient *redis.Client, prefix string, maxAttempts int64, window time.Duration) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ip := ClientIPFromContext(ctx.Context())
		key := fmt.Sprintf("rl:%s:%s", prefix, ip)

		count, err := redisClient.Incr(ctx.Context(), key).Result()
		if err != nil {
			slog.Warn("rate limiter: redis error, allowing request", "prefix", prefix, "error", err)
			next(ctx)
			return
		}
		if count == 1 {
			redisClient.Expire(ctx.Context(), key, window)
		}

		if count > maxAttempts {
			huma.WriteErr(api, ctx, http.StatusTooManyRequests, "too many requests, please try again later")
			return
		}

		next(ctx)
	}
}
