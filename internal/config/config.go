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

	// Password policy
	PasswordMinLength    int  // minimum password length (default 8)
	PasswordRequireUpper bool // require uppercase letter (default true)
	PasswordRequireLower bool // require lowercase letter (default true)
	PasswordRequireDigit bool // require digit (default true)

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
