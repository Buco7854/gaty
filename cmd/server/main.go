package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Buco7854/gaty/internal/cache"
	"github.com/Buco7854/gaty/internal/config"
	"github.com/Buco7854/gaty/internal/db"
	"github.com/Buco7854/gaty/internal/handler"
	"github.com/Buco7854/gaty/internal/middleware"
	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	redisClient, err := cache.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	slog.Info("redis connected")

	router := chi.NewMux()
	router.Use(chimw.RequestID)
	router.Use(chimw.RealIP)
	router.Use(chimw.Logger)
	router.Use(chimw.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Use(middleware.TenantResolver(pool))

	// MQTT client (non-fatal: API works without broker)
	mqttClient, err := internalmqtt.New(cfg.MQTTBroker)
	if err != nil {
		slog.Warn("mqtt unavailable, continuing without MQTT", "error", err)
	} else {
		defer mqttClient.Disconnect()
	}

	// Repositories
	userRepo := repository.NewUserRepository(pool)
	credRepo := repository.NewCredentialRepository(pool)
	wsRepo := repository.NewWorkspaceRepository(pool)
	gateRepo := repository.NewGateRepository(pool)
	policyRepo := repository.NewPolicyRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)

	// Subscribe to gate status updates from MQTT
	if mqttClient != nil {
		if err := mqttClient.SubscribeGateStatuses(gateRepo); err != nil {
			slog.Warn("mqtt: failed to subscribe to gate statuses", "error", err)
		}
	}

	// Services
	authSvc := service.NewAuthService(userRepo, credRepo, redisClient, cfg.JWTSecret)

	api := humachi.New(router, huma.DefaultConfig("GATY API", "0.1.0"))

	// Global soft auth middleware: silently extracts Bearer token and sets user ID in context
	api.UseMiddleware(middleware.AuthExtractor(authSvc))
	// Per-operation middlewares
	requireAuth := middleware.RequireAuth(api)
	wsMember := middleware.WorkspaceMember(api, wsRepo)
	wsAdmin := middleware.WorkspaceAdmin(api, wsRepo)

	type HealthOutput struct {
		Body struct {
			Status   string `json:"status"`
			Database string `json:"database"`
			Redis    string `json:"redis"`
		}
	}
	huma.Get(api, "/api/health", func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		resp := &HealthOutput{}
		resp.Body.Status = "ok"
		if err := pool.Ping(ctx); err != nil {
			resp.Body.Database = "unreachable"
		} else {
			resp.Body.Database = "ok"
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			resp.Body.Redis = "unreachable"
		} else {
			resp.Body.Redis = "ok"
		}
		return resp, nil
	})

	// Register route groups
	handler.NewAuthHandler(authSvc, userRepo).RegisterRoutes(api, requireAuth)
	handler.NewWorkspaceHandler(wsRepo).RegisterRoutes(api, requireAuth, wsAdmin)
	handler.NewGateHandler(gateRepo, policyRepo, auditRepo, mqttClient).RegisterRoutes(api, wsMember, wsAdmin)
	handler.NewPolicyHandler(policyRepo).RegisterRoutes(api, wsAdmin)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}
	slog.Info("server stopped")
}
