package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
)

// GateInboundHandler handles status reports pushed by gates over HTTP.
// Authentication is two-step: JWT signature verification + DB rotation check.
// The gate token alone identifies the gate — no workspace or gate ID needed in the path.
// A successful status push also acts as a keepalive (updates last_seen_at).
type GateInboundHandler struct {
	gates *service.GateService
}

func NewGateInboundHandler(gates *service.GateService) *GateInboundHandler {
	return &GateInboundHandler{gates: gates}
}

// --- POST /api/inbound/status ---

type GateStatusPushInput struct {
	Authorization string `header:"Authorization" required:"true"`
	Body          struct {
		// Status is the system-level state of the gate.
		// Well-known values: "open", "closed", "online", "offline".
		Status string `json:"status" minLength:"1"`
		// Meta holds arbitrary sensor/protocol metadata.
		// Keys and their meaning are defined by the gate's meta_config.
		// Supports nested objects with dot-notated keys in meta_config
		// (e.g. key "lora.snr" resolves {"lora": {"snr": -10.5}}).
		// Flat keys like {"battery": 85} also work.
		Meta map[string]any `json:"meta,omitempty"`
	}
}

func extractBearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}

func (h *GateInboundHandler) PushStatus(ctx context.Context, input *GateStatusPushInput) (*struct{}, error) {
	token := extractBearerToken(input.Authorization)
	if token == "" {
		return nil, huma.Error401Unauthorized("missing gate token")
	}

	// AuthenticateToken: verifies JWT signature then checks DB (rotation detection).
	gate, err := h.gates.AuthenticateToken(ctx, token)
	if err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid gate token")
		}
		return nil, huma.Error500InternalServerError("failed to authenticate gate")
	}

	if err := h.gates.ProcessStatus(ctx, gate, input.Body.Status, input.Body.Meta); err != nil {
		return nil, huma.Error500InternalServerError("failed to update status")
	}

	slog.Info("inbound: gate status updated", "gate_id", gate.ID, "status", input.Body.Status)
	return nil, nil
}

func (h *GateInboundHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "gate-inbound-status",
		Method:      http.MethodPost,
		Path:        "/api/inbound/status",
		Summary:     "Gate reports its current status (authenticated by gate token)",
		Description: "Used by HTTP-mode gates to report their state to the server. " +
			"Authentication is two-step: JWT signature verification + DB rotation check. " +
			"The gate token alone identifies the gate — no gate_id needed in the path. " +
			"Each successful call also acts as a keepalive (updates last_seen_at). " +
			"Authentication: Authorization: Bearer {gate_token}.",
		Tags: []string{"Gate Inbound"},
	}, h.PushStatus)
}
