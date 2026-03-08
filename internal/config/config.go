package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  int
	DatabaseURL           string
	RedisURL              string
	MQTTBroker            string
	MQTTUsername          string // optional, empty = anonymous
	MQTTPassword          string // optional
	JWTSecret             string
	CORSOrigins           []string
	BaseURL               string        // public base URL of this API, e.g. https://api.example.com
	FrontendURL           string        // public URL of the frontend SPA, for post-SSO redirects
	GlobalSessionDuration time.Duration // refresh token TTL for platform users (0 = infinite)
	CookieSecure          bool          // set Secure flag on auth cookies (true when BASE_URL is https)

	// JWT / session settings
	AccessTokenTTL time.Duration // access token lifetime (default 15 min)
	BcryptCost     int           // bcrypt work factor (default 12 / bcrypt.DefaultCost)

	// Password policy
	PasswordMinLength    int  // minimum password length (default 8)
	PasswordRequireUpper bool // require uppercase letter (default true)
	PasswordRequireLower bool // require lowercase letter (default true)
	PasswordRequireDigit bool // require digit (default true)

	// PIN unlock settings
	PINMinLength       int           // minimum digits for a numeric PIN code (default 4)
	PINMaxAttempts     int64         // max failed attempts per IP per window (default 5)
	PINGateMaxAttempts int64         // max failed attempts per gate per window (default 50)
	PINRateLimitWindow time.Duration // sliding window for PIN rate limiting (default 15 min)
	PINMinUnlockMs     int           // minimum response time for unlock endpoints in ms (default 400)

	// SSE settings
	SSETicketTTL time.Duration // one-time SSE ticket lifetime (default 30 s)

	// Tenant domain cache settings
	TenantCacheTTL         time.Duration // TTL for verified custom domain entries (default 5 min)
	TenantCacheNotFoundTTL time.Duration // TTL for negative-result entries (default 30 s)

	// HTTP driver settings (gate open/close and webhook status polling)
	HTTPDriverDialTimeoutMs     int // TCP connect + TLS timeout in ms (default 5000)
	HTTPDriverResponseTimeoutMs int // response header timeout in ms (default 10000)

	// Webhook poller settings (HTTP_WEBHOOK status mode).
	WebhookMaxRetries int           // max retry attempts per poll (default 3)
	WebhookRetryDelay time.Duration // delay between retries (default 1s)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:        8080,
		CORSOrigins: []string{"http://localhost:5173"},
	}

	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		cfg.Port = p
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg.RedisURL = os.Getenv("REDIS_URL")
	if cfg.RedisURL == "" {
		cfg.RedisURL = "redis://localhost:6379"
	}

	cfg.MQTTBroker = os.Getenv("MQTT_BROKER")
	if cfg.MQTTBroker == "" {
		cfg.MQTTBroker = "tcp://localhost:1883"
	}

	cfg.MQTTUsername = os.Getenv("MQTT_USERNAME")
	cfg.MQTTPassword = os.Getenv("MQTT_PASSWORD")

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		origins := strings.Split(v, ",")
		for _, o := range origins {
			o = strings.TrimSpace(o)
			if o == "*" {
				return nil, fmt.Errorf("CORS_ORIGINS must not contain wildcard '*' (credentials mode requires explicit origins)")
			}
		}
		cfg.CORSOrigins = origins
	}

	cfg.BaseURL = os.Getenv("BASE_URL")
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}

	cfg.FrontendURL = os.Getenv("FRONTEND_URL")
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:5173"
	}

	cfg.CookieSecure = strings.HasPrefix(cfg.BaseURL, "https://")

	// Password policy: defaults are strict; relax via env vars.
	cfg.PasswordMinLength = 8
	cfg.PasswordRequireUpper = true
	cfg.PasswordRequireLower = true
	cfg.PasswordRequireDigit = true
	if v := os.Getenv("AUTH_PASSWORD_MIN_LENGTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid AUTH_PASSWORD_MIN_LENGTH: %w", err)
		}
		cfg.PasswordMinLength = n
	}
	if v := os.Getenv("AUTH_PASSWORD_REQUIRE_UPPER"); v != "" {
		cfg.PasswordRequireUpper = v == "true" || v == "1"
	}
	if v := os.Getenv("AUTH_PASSWORD_REQUIRE_LOWER"); v != "" {
		cfg.PasswordRequireLower = v == "true" || v == "1"
	}
	if v := os.Getenv("AUTH_PASSWORD_REQUIRE_DIGIT"); v != "" {
		cfg.PasswordRequireDigit = v == "true" || v == "1"
	}

	// AUTH_SESSION_DURATION: refresh token lifetime for platform users, in seconds.
	// 0 means infinite (no expiry). Default: 7 days.
	cfg.GlobalSessionDuration = 7 * 24 * time.Hour
	if v := os.Getenv("AUTH_SESSION_DURATION"); v != "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid AUTH_SESSION_DURATION: %w", err)
		}
		if secs == 0 {
			cfg.GlobalSessionDuration = 0
		} else {
			cfg.GlobalSessionDuration = time.Duration(secs) * time.Second
		}
	}

	// AUTH_ACCESS_TOKEN_TTL: access token lifetime in seconds. Default: 900 (15 min).
	cfg.AccessTokenTTL = 15 * time.Minute
	if v := os.Getenv("AUTH_ACCESS_TOKEN_TTL"); v != "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs <= 0 {
			return nil, fmt.Errorf("invalid AUTH_ACCESS_TOKEN_TTL: must be a positive integer (seconds)")
		}
		cfg.AccessTokenTTL = time.Duration(secs) * time.Second
	}

	// AUTH_BCRYPT_COST: bcrypt work factor for password hashing. Default: 12.
	// Valid range: 4–31. Higher values are more secure but slower.
	cfg.BcryptCost = 12
	if v := os.Getenv("AUTH_BCRYPT_COST"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 4 || n > 31 {
			return nil, fmt.Errorf("invalid AUTH_BCRYPT_COST: must be an integer between 4 and 31")
		}
		cfg.BcryptCost = n
	}

	// PIN unlock settings.
	cfg.PINMinLength = 4
	if v := os.Getenv("PIN_MIN_LENGTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid PIN_MIN_LENGTH: must be a positive integer")
		}
		cfg.PINMinLength = n
	}

	cfg.PINMaxAttempts = 5
	if v := os.Getenv("PIN_MAX_ATTEMPTS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid PIN_MAX_ATTEMPTS: must be a positive integer")
		}
		cfg.PINMaxAttempts = n
	}

	cfg.PINGateMaxAttempts = 50
	if v := os.Getenv("PIN_GATE_MAX_ATTEMPTS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid PIN_GATE_MAX_ATTEMPTS: must be a positive integer")
		}
		cfg.PINGateMaxAttempts = n
	}

	cfg.PINRateLimitWindow = 15 * time.Minute
	if v := os.Getenv("PIN_RATE_LIMIT_WINDOW"); v != "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs <= 0 {
			return nil, fmt.Errorf("invalid PIN_RATE_LIMIT_WINDOW: must be a positive integer (seconds)")
		}
		cfg.PINRateLimitWindow = time.Duration(secs) * time.Second
	}

	cfg.PINMinUnlockMs = 400
	if v := os.Getenv("PIN_MIN_UNLOCK_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid PIN_MIN_UNLOCK_MS: must be a non-negative integer")
		}
		cfg.PINMinUnlockMs = n
	}

	// SSE_TICKET_TTL: one-time SSE ticket lifetime in seconds. Default: 30.
	cfg.SSETicketTTL = 30 * time.Second
	if v := os.Getenv("SSE_TICKET_TTL"); v != "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs <= 0 {
			return nil, fmt.Errorf("invalid SSE_TICKET_TTL: must be a positive integer (seconds)")
		}
		cfg.SSETicketTTL = time.Duration(secs) * time.Second
	}

	// TENANT_CACHE_TTL: Redis cache lifetime for verified custom domain entries, in seconds. Default: 300.
	cfg.TenantCacheTTL = 5 * time.Minute
	if v := os.Getenv("TENANT_CACHE_TTL"); v != "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs <= 0 {
			return nil, fmt.Errorf("invalid TENANT_CACHE_TTL: must be a positive integer (seconds)")
		}
		cfg.TenantCacheTTL = time.Duration(secs) * time.Second
	}

	// TENANT_CACHE_NOT_FOUND_TTL: Redis cache lifetime for negative domain lookups, in seconds. Default: 30.
	cfg.TenantCacheNotFoundTTL = 30 * time.Second
	if v := os.Getenv("TENANT_CACHE_NOT_FOUND_TTL"); v != "" {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs <= 0 {
			return nil, fmt.Errorf("invalid TENANT_CACHE_NOT_FOUND_TTL: must be a positive integer (seconds)")
		}
		cfg.TenantCacheNotFoundTTL = time.Duration(secs) * time.Second
	}

	// HTTP_DRIVER_DIAL_TIMEOUT_MS: TCP connect + TLS handshake timeout for gate HTTP drivers, in ms. Default: 5000.
	cfg.HTTPDriverDialTimeoutMs = 5000
	if v := os.Getenv("HTTP_DRIVER_DIAL_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid HTTP_DRIVER_DIAL_TIMEOUT_MS: must be a positive integer")
		}
		cfg.HTTPDriverDialTimeoutMs = n
	}

	// HTTP_DRIVER_RESPONSE_TIMEOUT_MS: response header timeout for gate HTTP drivers, in ms. Default: 10000.
	cfg.HTTPDriverResponseTimeoutMs = 10000
	if v := os.Getenv("HTTP_DRIVER_RESPONSE_TIMEOUT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid HTTP_DRIVER_RESPONSE_TIMEOUT_MS: must be a positive integer")
		}
		cfg.HTTPDriverResponseTimeoutMs = n
	}

	// WEBHOOK_MAX_RETRIES: number of retry attempts for HTTP_WEBHOOK status polls. Default: 3.
	cfg.WebhookMaxRetries = 3
	if v := os.Getenv("WEBHOOK_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WEBHOOK_MAX_RETRIES: %w", err)
		}
		cfg.WebhookMaxRetries = n
	}

	// WEBHOOK_RETRY_DELAY_MS: milliseconds between webhook poll retries. Default: 1000.
	cfg.WebhookRetryDelay = time.Second
	if v := os.Getenv("WEBHOOK_RETRY_DELAY_MS"); v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WEBHOOK_RETRY_DELAY_MS: %w", err)
		}
		cfg.WebhookRetryDelay = time.Duration(ms) * time.Millisecond
	}

	return cfg, nil
}
