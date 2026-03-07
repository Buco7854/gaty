package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// SSEHandler streams gate events to authenticated clients via Server-Sent Events.
// Implemented as a raw chi handler (not Huma) to support long-lived connections.
type SSEHandler struct {
	authSvc *service.AuthService
	redis   *redis.Client
}

func NewSSEHandler(authSvc *service.AuthService, redisClient *redis.Client) *SSEHandler {
	return &SSEHandler{authSvc: authSvc, redis: redisClient}
}

func (h *SSEHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/workspaces/{ws_id}/events", h.stream)
}

func (h *SSEHandler) stream(w http.ResponseWriter, r *http.Request) {
	// Extract token from Authorization header or ?token= query param.
	// Query param is supported for EventSource compatibility but discouraged
	// because tokens in URLs leak via logs, browser history, and Referer headers.
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
		if token != "" {
			slog.Warn("sse: token passed via query parameter, prefer Authorization header")
		}
	}
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	wsID := chi.URLParam(r, "ws_id")

	// Validate token: accept global (user) or local (member) JWT.
	_, globalErr := h.authSvc.ValidateAccessToken(token)
	if globalErr != nil {
		_, tokenWSID, _, localErr := h.authSvc.ValidateMemberToken(token)
		if localErr != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Local token: workspace must match.
		if tokenWSID.String() != wsID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("sse: response writer does not support flushing")
		return
	}

	ctx := r.Context()
	channel := fmt.Sprintf("gate:events:%s", wsID)
	pubsub := h.redis.Subscribe(ctx, channel)
	defer pubsub.Close()

	redisCh := pubsub.Channel()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial comment to establish the stream.
	fmt.Fprintf(w, ": connected to %s\n\n", channel)
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-redisCh:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
