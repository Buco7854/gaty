package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Buco7854/gatie/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

const sseTicketPrefix = "sse:ticket:"

// SSEHandler streams gate events to authenticated clients via Server-Sent Events.
// Implemented as a raw chi handler (not Huma) to support long-lived connections.
type SSEHandler struct {
	authSvc       *service.AuthService
	redis         *redis.Client
	sseTicketTTL  time.Duration
}

func NewSSEHandler(authSvc *service.AuthService, redisClient *redis.Client, ticketTTL time.Duration) *SSEHandler {
	if ticketTTL <= 0 {
		ticketTTL = 30 * time.Second
	}
	return &SSEHandler{authSvc: authSvc, redis: redisClient, sseTicketTTL: ticketTTL}
}

func (h *SSEHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/workspaces/{ws_id}/events/ticket", h.issueTicket)
	r.Get("/api/workspaces/{ws_id}/events", h.stream)
}

// issueTicket authenticates via gatie_access cookie and returns a one-time ticket for SSE.
func (h *SSEHandler) issueTicket(w http.ResponseWriter, r *http.Request) {
	var token string
	if c, err := r.Cookie("gatie_access"); err == nil {
		token = c.Value
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
		if tokenWSID.String() != wsID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ticket := hex.EncodeToString(b)

	if err := h.redis.Set(r.Context(), sseTicketPrefix+ticket, wsID, h.sseTicketTTL).Err(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ticket": ticket})
}

func (h *SSEHandler) stream(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "ws_id")

	// Authenticate via one-time ticket (preferred) or Authorization header.
	authenticated := false
	if ticket := r.URL.Query().Get("ticket"); ticket != "" {
		ticketWSID, err := h.redis.GetDel(r.Context(), sseTicketPrefix+ticket).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			} else {
				slog.Error("sse: redis error during ticket validation", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
			return
		}
		if ticketWSID != wsID {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		authenticated = true
	}

	if !authenticated {
		var token string
		if c, err := r.Cookie("gatie_access"); err == nil {
			token = c.Value
		}
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, globalErr := h.authSvc.ValidateAccessToken(token)
		if globalErr != nil {
			_, tokenWSID, _, localErr := h.authSvc.ValidateMemberToken(token)
			if localErr != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if tokenWSID.String() != wsID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
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
