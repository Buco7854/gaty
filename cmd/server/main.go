package main

import (
	"context"
	"errors"
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
	router.Use(middleware.CookieFixer())
	router.Use(middleware.TenantResolver(pool, redisClient))

	// MQTT client (non-fatal: API works without broker)
	mqttClient, err := internalmqtt.New(cfg.MQTTBroker, cfg.MQTTUsername, cfg.MQTTPassword)
	if err != nil {
		slog.Warn("mqtt unavailable, continuing without MQTT", "error", err)
	} else {
		defer mqttClient.Disconnect()
	}

	// Repositories
	memberRepo := repopg.NewMemberRepository(pool)
	credRepo := repopg.NewCredentialRepository(pool)
	gateRepo := repopg.NewGateRepository(pool)
	gatePinRepo := repopg.NewGatePinRepository(pool)
	policyRepo := repopg.NewPolicyRepository(pool)
	credPolicyRepo := repopg.NewCredentialPolicyRepository(pool)
	scheduleRepo := repopg.NewAccessScheduleRepository(pool)
	auditRepo := repopg.NewAuditRepository(pool)
	domainRepo := repopg.NewCustomDomainRepository(pool)

	// MQTT auth strategy: DynSec (broker-level) or payload (app-level fallback).
	var brokerAuth service.BrokerAuthManager = service.NoopBrokerAuth{}
	brokerAuthEnabled := cfg.MQTTAuthMode == "dynsec"
	if brokerAuthEnabled && mqttClient != nil {
		dynSec := internalmqtt.NewDynSecManager(mqttClient)
		if err := dynSec.Setup(ctx); err != nil {
			slog.Warn("dynsec: setup failed, falling back to payload auth", "error", err)
			brokerAuthEnabled = false
		} else {
			brokerAuth = dynSec
		}
	}

	// Subscribe to gate status updates from MQTT (bridge to Redis Pub/Sub for SSE)
	if mqttClient != nil {
		if err := mqttClient.SubscribeGateStatuses(gateRepo, redisClient, brokerAuthEnabled, []byte(cfg.JWTSecret)); err != nil {
			slog.Warn("mqtt: failed to subscribe to gate statuses", "error", err)
		}
	}

	// Services
	authSvc := service.NewAuthService(memberRepo, credRepo, redisClient, cfg.JWTSecret, cfg.GlobalSessionDuration, service.PasswordPolicy{
		MinLength:    cfg.PasswordMinLength,
		RequireUpper: cfg.PasswordRequireUpper,
		RequireLower: cfg.PasswordRequireLower,
		RequireDigit: cfg.PasswordRequireDigit,
	})
	memberSvc := service.NewMemberService(memberRepo, credRepo)

	// SSO providers from env
	var ssoProviders []service.SSOProviderConfig
	if cfg.OIDCClientID != "" {
		ssoProviders = append(ssoProviders, service.SSOProviderConfig{
			ID:            cfg.OIDCProviderID,
			Name:          cfg.OIDCProviderName,
			Type:          "oidc",
			ClientID:      cfg.OIDCClientID,
			ClientSecret:  cfg.OIDCClientSecret,
			Issuer:        cfg.OIDCIssuer,
			Scopes:        cfg.OIDCScopes,
			AuthEndpoint:  cfg.OIDCAuthEndpoint,
			TokenEndpoint: cfg.OIDCTokenEndpoint,
			JwksURL:       cfg.OIDCJwksURL,
			AutoProvision: cfg.OIDCAutoProvision,
			DefaultRole:   model.Role(cfg.OIDCDefaultRole),
			RoleClaim:     cfg.OIDCRoleClaim,
			RoleMapping:   cfg.OIDCRoleMapping,
		})
	}
	ssoSvc := service.NewSSOService(memberRepo, credRepo, redisClient, cfg.BaseURL, ssoProviders)

	scheduleSvc := service.NewScheduleService(scheduleRepo)
	policySvc := service.NewPolicyService(policyRepo, scheduleRepo)

	// Shared SSRF-safe HTTP client for all outbound gate HTTP requests.
	gateHTTPClient := integration.NewHTTPClient(cfg.HTTPDriverAllowedCIDRs)

	// gateTrigger fires open/close drivers; defined here to avoid an import cycle.
	gateTrigger := service.GateTriggerFn(func(ctx context.Context, gate *model.Gate, action string) {
		var driver integration.Driver
		var err error
		if action == "close" {
			driver, err = integration.NewCloseDriver(gate, mqttClient, gateHTTPClient)
		} else {
			driver, err = integration.NewOpenDriver(gate, mqttClient, gateHTTPClient)
		}
		if err != nil {
			slog.Warn("gate: failed to build driver", "gate_id", gate.ID, "action", action, "error", err)
			return
		}
		if err := driver.Execute(ctx, gate); err != nil {
			slog.Warn("gate: driver execution failed", "gate_id", gate.ID, "action", action, "error", err)
		}
	})
	gateSvc := service.NewGateService(gateRepo, policySvc, credPolicyRepo, scheduleSvc, auditRepo, gateTrigger, []byte(cfg.JWTSecret), redisClient, brokerAuth)
	gatePinSvc := service.NewGatePinService(gatePinRepo, gateRepo, scheduleSvc, authSvc, gateTrigger, redisClient, cfg.PINMaxAttempts, cfg.PINGateMaxAttempts)

	// Gate TTL worker: marks gates unresponsive when last_seen_at > DefaultGateTTL ago.
	ttlCtx, ttlCancel := context.WithCancel(ctx)
	defer ttlCancel()
	go service.NewGateTTLWorker(gateRepo, redisClient, service.DefaultGateTTL).Run(ttlCtx)

	// Webhook worker: polls HTTP_WEBHOOK-configured gates on their configured interval.
	webhookCtx, webhookCancel := context.WithCancel(ctx)
	defer webhookCancel()
	go service.NewGateWebhookWorker(gateRepo, redisClient, cfg.WebhookMaxRetries, cfg.WebhookRetryDelay, gateHTTPClient).Run(webhookCtx)

	// Global error hook: log 5xx with the original cause, but never expose raw
	// errors to the client.
	huma.NewError = func(status int, message string, errs ...error) huma.StatusError {
		m := &huma.ErrorModel{Status: status, Title: http.StatusText(status), Detail: message}
		for _, err := range errs {
			if err == nil {
				continue
			}
			var detail *huma.ErrorDetail
			if errors.As(err, &detail) {
				m.Errors = append(m.Errors, detail)
			} else if status >= 500 {
				slog.Error(message, "error", err)
			}
		}
		return m
	}

	api := humachi.New(router, huma.DefaultConfig("GATIE API", "0.1.0"))

	// Global soft auth middleware: silently extracts Bearer token and injects identity into context.
	api.UseMiddleware(middleware.AuthExtractor(authSvc, credRepo))

	// Rate limiters (fail-closed via Redis)
	authRateLimit := middleware.RateLimiter(api, redisClient, "auth", 10, 10*time.Minute)
	ssoExchangeRateLimit := middleware.RateLimiter(api, redisClient, "sso-exchange", 10, 10*time.Minute)

	// Per-operation middlewares
	requireAuth := middleware.RequireAuth(api)
	requireAdmin := middleware.RequireAdmin(api)
	gateManager := middleware.GateManager(api, policyRepo)
	adminOrGateManager := middleware.AdminOrGateManager(api, policyRepo)

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
	handler.NewSetupHandler(memberSvc, authSvc, cfg.CookieSecure).RegisterRoutes(api)

	// Register route groups
	handler.NewAuthHandler(authSvc, memberSvc, cfg.CookieSecure).RegisterRoutes(api, requireAuth, authRateLimit)
	handler.NewGateHandler(gateSvc).RegisterRoutes(api, requireAuth, requireAdmin, gateManager)
	handler.NewPolicyHandler(policySvc).RegisterRoutes(api, requireAuth, requireAdmin, gateManager)
	handler.NewMemberHandler(memberSvc).RegisterRoutes(api, requireAdmin)
	handler.NewGatePinHandler(gatePinSvc, cfg.CookieSecure).RegisterRoutes(api, requireAuth, gateManager)
	handler.NewAccessScheduleHandler(scheduleSvc).RegisterRoutes(api, requireAuth, requireAdmin, adminOrGateManager)
	handler.NewSSOHandler(ssoSvc, authSvc, redisClient, cfg.FrontendURL, cfg.CookieSecure).RegisterRoutes(api, requireAdmin, ssoExchangeRateLimit)
	handler.NewCredentialHandler(credRepo, memberRepo, credPolicyRepo).RegisterRoutes(api, requireAuth, requireAdmin)
	handler.NewCustomDomainHandler(domainRepo, gateRepo).RegisterRoutes(api, requireAuth, gateManager)

	// Inbound: gate-to-server status push (gate token auth).
	handler.NewGateInboundHandler(gateSvc).RegisterRoutes(api)

	// SSE: raw chi route (long-lived, not Huma)
	handler.NewSSEHandler(authSvc, redisClient).RegisterRoutes(router)

	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Port),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
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
