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

	"github.com/Buco7854/gatie/internal/cache"
	"github.com/Buco7854/gatie/internal/config"
	"github.com/Buco7854/gatie/internal/db"
	"github.com/Buco7854/gatie/internal/handler"
	"github.com/Buco7854/gatie/internal/integration"
	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	internalmqtt "github.com/Buco7854/gatie/internal/mqtt"
	repopg "github.com/Buco7854/gatie/internal/repository/postgres"
	"github.com/Buco7854/gatie/internal/service"
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
	router.Use(middleware.ClientIPInjector())
	router.Use(chimw.Logger)
	router.Use(middleware.JSONRecoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Use(middleware.TenantResolver(pool, redisClient))

	// MQTT client (non-fatal: API works without broker)
	mqttClient, err := internalmqtt.New(cfg.MQTTBroker, cfg.MQTTUsername, cfg.MQTTPassword)
	if err != nil {
		slog.Warn("mqtt unavailable, continuing without MQTT", "error", err)
	} else {
		defer mqttClient.Disconnect()
	}

	// Repositories
	userRepo := repopg.NewUserRepository(pool)
	credRepo := repopg.NewCredentialRepository(pool)
	wsRepo := repopg.NewWorkspaceRepository(pool)
	membershipRepo := repopg.NewWorkspaceMembershipRepository(pool)
	memberCredRepo := repopg.NewMembershipCredentialRepository(pool)
	gateRepo := repopg.NewGateRepository(pool)
	gatePinRepo := repopg.NewGatePinRepository(pool)
	policyRepo := repopg.NewPolicyRepository(pool)
	credPolicyRepo := repopg.NewCredentialPolicyRepository(pool)
	scheduleRepo := repopg.NewAccessScheduleRepository(pool)
	auditRepo := repopg.NewAuditRepository(pool)
	domainRepo := repopg.NewCustomDomainRepository(pool)

	// Subscribe to gate status updates from MQTT (bridge to Redis Pub/Sub for SSE)
	if mqttClient != nil {
		if err := mqttClient.SubscribeGateStatuses(gateRepo, redisClient, []byte(cfg.JWTSecret)); err != nil {
			slog.Warn("mqtt: failed to subscribe to gate statuses", "error", err)
		}
	}

	// Services
	authSvc := service.NewAuthService(userRepo, credRepo, membershipRepo, memberCredRepo, wsRepo, redisClient, cfg.JWTSecret, cfg.GlobalSessionDuration)
	membershipSvc := service.NewMembershipService(membershipRepo, memberCredRepo, wsRepo)
	ssoSvc := service.NewSSOService(wsRepo, membershipRepo, memberCredRepo, redisClient, cfg.BaseURL)
	workspaceSvc := service.NewWorkspaceService(wsRepo)
	scheduleSvc := service.NewScheduleService(scheduleRepo)
	policySvc := service.NewPolicyService(policyRepo, scheduleRepo)
	// gateTrigger fires open/close drivers; defined here to avoid an import cycle
	// (service -> integration -> mqtt -> service).
	gateTrigger := service.GateTriggerFn(func(ctx context.Context, gate *model.Gate, action string) {
		var driver integration.Driver
		var err error
		if action == "close" {
			driver, err = integration.NewCloseDriver(gate, mqttClient)
		} else {
			driver, err = integration.NewOpenDriver(gate, mqttClient)
		}
		if err != nil {
			slog.Warn("gate: failed to build driver", "gate_id", gate.ID, "action", action, "error", err)
			return
		}
		if err := driver.Execute(ctx, gate); err != nil {
			slog.Warn("gate: driver execution failed", "gate_id", gate.ID, "action", action, "error", err)
		}
	})
	gateSvc := service.NewGateService(gateRepo, policySvc, credPolicyRepo, scheduleSvc, auditRepo, gateTrigger, []byte(cfg.JWTSecret), redisClient)
	gatePinSvc := service.NewGatePinService(gatePinRepo, gateRepo, scheduleSvc, authSvc, gateTrigger, redisClient)

	// Gate TTL worker: marks gates unresponsive when last_seen_at > DefaultGateTTL ago.
	ttlCtx, ttlCancel := context.WithCancel(ctx)
	defer ttlCancel()
	go service.NewGateTTLWorker(gateRepo, redisClient, service.DefaultGateTTL).Run(ttlCtx)

	api := humachi.New(router, huma.DefaultConfig("GATIE API", "0.1.0"))

	// Global soft auth middleware: silently extracts Bearer token and injects identity into context.
	api.UseMiddleware(middleware.AuthExtractor(authSvc, memberCredRepo))

	// Per-operation middlewares
	requireAuth := middleware.RequireAuth(api)
	requireMembership := middleware.RequireMembership(api)
	wsMember := middleware.WorkspaceMember(api, wsRepo, membershipRepo)
	wsAdmin := middleware.WorkspaceAdmin(api, wsRepo, membershipRepo)
	wsGateManager := middleware.GateManager(api, policyRepo)

	huma.Get(api, "/api/health", func(ctx context.Context, _ *struct{}) (*struct {
		Body struct {
			Status   string `json:"status"`
			Database string `json:"database"`
			Redis    string `json:"redis"`
		}
	}, error) {
		resp := &struct {
			Body struct {
				Status   string `json:"status"`
				Database string `json:"database"`
				Redis    string `json:"redis"`
			}
		}{}
		resp.Body.Status = "ok"
		hctx, hcancel := context.WithTimeout(ctx, 3*time.Second)
		defer hcancel()
		if err := pool.Ping(hctx); err != nil {
			resp.Body.Database = "unreachable"
		} else {
			resp.Body.Database = "ok"
		}
		if err := redisClient.Ping(hctx).Err(); err != nil {
			resp.Body.Redis = "unreachable"
		} else {
			resp.Body.Redis = "ok"
		}
		return resp, nil
	})

	// Setup (public, no auth)
	handler.NewSetupHandler(userRepo, authSvc).RegisterRoutes(api)

	// Register route groups
	handler.NewAuthHandler(authSvc, userRepo).RegisterRoutes(api, requireAuth)
	handler.NewWorkspaceHandler(workspaceSvc).RegisterRoutes(api, requireAuth, wsAdmin)
	handler.NewGateHandler(gateSvc).RegisterRoutes(api, wsMember, wsAdmin, wsGateManager)
	handler.NewPolicyHandler(policySvc).RegisterRoutes(api, wsMember, wsAdmin, wsGateManager)
	handler.NewMemberHandler(membershipSvc).RegisterRoutes(api, wsAdmin)
	handler.NewGatePinHandler(gatePinSvc).RegisterRoutes(api, wsMember, wsGateManager)
	handler.NewAccessScheduleHandler(scheduleSvc).RegisterRoutes(api, wsAdmin)
	handler.NewSSOHandler(ssoSvc, authSvc, wsRepo, cfg.FrontendURL).RegisterRoutes(api, wsAdmin)
	handler.NewCredentialHandler(credRepo, memberCredRepo, membershipRepo, credPolicyRepo).RegisterRoutes(api, requireAuth, requireMembership, wsMember, wsAdmin)
	handler.NewCustomDomainHandler(domainRepo, gateRepo).RegisterRoutes(api, wsMember, wsGateManager)

	// Inbound: gate-to-server status push (gate token auth, no workspace middleware).
	handler.NewGateInboundHandler(gateSvc).RegisterRoutes(api)

	// SSE: raw chi route (long-lived, not Huma)
	handler.NewSSEHandler(authSvc, redisClient).RegisterRoutes(router)

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
