package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         int
	DatabaseURL  string
	RedisURL     string
	MQTTBroker   string
	MQTTUsername string // optional, empty = anonymous
	MQTTPassword string // optional
	JWTSecret    string
	CORSOrigins  []string
	BaseURL      string // public base URL of this API, e.g. https://api.example.com
	FrontendURL  string // public URL of the frontend SPA, for post-SSO redirects
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

	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = strings.Split(v, ",")
	}

	cfg.BaseURL = os.Getenv("BASE_URL")
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
	}

	cfg.FrontendURL = os.Getenv("FRONTEND_URL")
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "http://localhost:5173"
	}

	return cfg, nil
}
