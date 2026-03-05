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
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// GateInboundHandler handles status reports pushed by gates over HTTP.
// Authentication is performed using the gate's unique gate_token.
type GateInboundHandler struct {
	gates *repository.GateRepository
	redis *redis.Client
}

func NewGateInboundHandler(gates *repository.GateRepository, redis *redis.Client) *GateInboundHandler {
	return &GateInboundHandler{gates: gates, redis: redis}
}

// --- POST /api/inbound/gates/{gate_id}/status ---

type GateStatusPushInput struct {
	// GateID is extracted from the URL path.
	GateID uuid.UUID `path:"gate_id"`
	// Authorization must be "Bearer {gate_token}".
	Authorization string `header:"Authorization" required:"true"`
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

	if err := h.gates.UpdateStatusWithMeta(ctx, input.GateID, token, input.Body.Status, input.Body.Meta); err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid gate token")
		}
		return nil, huma.Error500InternalServerError("failed to update status")
	}

	// Publish SSE event via Redis Pub/Sub (same channel as MQTT path).
	if h.redis != nil {
		// We need the workspace_id for the event channel. Fetch the gate.
		gate, err := h.gates.GetByIDPublic(ctx, input.GateID)
		if err == nil {
			event := internalmqtt.GateEvent{
				GateID:         input.GateID.String(),
				WorkspaceID:    gate.WorkspaceID.String(),
				Status:         input.Body.Status,
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
	}

	slog.Info("inbound: gate status updated", "gate_id", input.GateID, "status", input.Body.Status)
	return nil, nil
}

// RegisterRoutes wires the inbound gate endpoint onto the Huma API.
// No workspace middleware — the gate authenticates with its own token.
func (h *GateInboundHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "gate-inbound-status",
		Method:      http.MethodPost,
		Path:        "/api/inbound/gates/{gate_id}/status",
		Summary:     "Gate pushes its current status (authenticated by gate token)",
		Description: "Used by HTTP-mode gates to report their state to the server. " +
			"Authentication: Authorization: Bearer {gate_token}.",
		Tags: []string{"Gate Inbound"},
	}, h.PushStatus)
}
