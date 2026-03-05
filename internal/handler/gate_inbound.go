package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// GateInboundHandler handles status reports pushed by gates over HTTP.
// Authentication is performed using the gate's unique gate_token.
// A successful status push also acts as a keepalive (updates last_seen_at).
type GateInboundHandler struct {
	gates repository.GateRepository
	redis *redis.Client
}

func NewGateInboundHandler(gates repository.GateRepository, redis *redis.Client) *GateInboundHandler {
	return &GateInboundHandler{gates: gates, redis: redis}
}

// --- POST /api/inbound/gates/{gate_id}/status ---

type GateStatusPushInput struct {
	GateID        uuid.UUID `path:"gate_id"`
	Authorization string    `header:"Authorization" required:"true"`
	Body          struct {
		// Status is the system-level state of the gate.
		// Well-known values: "open", "closed", "online", "offline".
		Status string `json:"status" minLength:"1"`
		// Meta holds arbitrary sensor/protocol metadata.
		// Keys and their meaning are defined by the gate's meta_config.
		// Example: {"lora.snr": -10.5, "lora.rssi": -90, "battery": 85}
		Meta map[string]any `json:"meta,omitempty"`
	}
}

func (h *GateInboundHandler) PushStatus(ctx context.Context, input *GateStatusPushInput) (*struct{}, error) {
	token := strings.TrimPrefix(input.Authorization, "Bearer ")
	if token == "" {
		return nil, huma.Error401Unauthorized("missing gate token")
	}

	// Validate token and retrieve gate (including status_rules for rule evaluation).
	gate, err := h.gates.GetByToken(ctx, input.GateID, token)
	if err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid gate token")
		}
		return nil, huma.Error500InternalServerError("failed to authenticate gate")
	}

	// Business logic: evaluate status rules against the incoming metadata.
	finalStatus := input.Body.Status
	if override, ok := model.EvaluateStatusRules(gate.StatusRules, input.Body.Meta); ok {
		finalStatus = override
	}

	if err := h.gates.UpdateStatus(ctx, input.GateID, finalStatus, input.Body.Meta); err != nil {
		return nil, huma.Error500InternalServerError("failed to update status")
	}

	if h.redis != nil {
		event := internalmqtt.GateEvent{
			GateID:         input.GateID.String(),
			WorkspaceID:    gate.WorkspaceID.String(),
			Status:         finalStatus,
			StatusMetadata: input.Body.Meta,
		}
		payload, _ := json.Marshal(event)
		channel := fmt.Sprintf("gate:events:%s", gate.WorkspaceID)
		tCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := h.redis.Publish(tCtx, channel, string(payload)).Err(); err != nil {
			slog.Warn("inbound: failed to publish gate event", "channel", channel, "error", err)
		}
	}

	slog.Info("inbound: gate status updated", "gate_id", input.GateID, "status", finalStatus)
	return nil, nil
}

func (h *GateInboundHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "gate-inbound-status",
		Method:      http.MethodPost,
		Path:        "/api/inbound/gates/{gate_id}/status",
		Summary:     "Gate reports its current status (authenticated by gate token)",
		Description: "Used by HTTP-mode gates to report their state to the server. " +
			"Each successful call also acts as a keepalive — it updates last_seen_at, " +
			"resetting the unresponsive TTL. " +
			"Authentication: Authorization: Bearer {gate_token}.",
		Tags: []string{"Gate Inbound"},
	}, h.PushStatus)
}
