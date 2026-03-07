package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Buco7854/gatie/internal/model"
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
	Authorization string         `header:"Authorization" required:"true"`
	Body          map[string]any `json:"body"`
}

func extractBearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}

// resolveInboundPayload extracts status and metadata from a raw HTTP inbound payload.
// The gate must have an HTTP_INBOUND StatusConfig with a PayloadMapping configured.
// Meta is extracted via gate.MetaConfig keys.
// Returns an error if no mapping is found or the mapping fails.
func resolveInboundPayload(gate *model.Gate, raw map[string]any) (status string, meta map[string]any, err error) {
	if gate.StatusConfig == nil ||
		gate.StatusConfig.Type != model.DriverTypeHTTPInbound ||
		len(gate.StatusConfig.Config) == 0 {
		return "", nil, fmt.Errorf("no HTTP_INBOUND status_config mapping configured")
	}
	mapping, ok := model.ExtractPayloadMapping(gate.StatusConfig.Config)
	if !ok {
		return "", nil, fmt.Errorf("status_config.config.mapping missing or invalid")
	}
	status, err = model.ApplyMapping(*mapping, raw)
	if err != nil {
		return "", nil, err
	}
	return status, model.ExtractMeta(gate.MetaConfig, raw), nil
}

func (h *GateInboundHandler) PushStatus(ctx context.Context, input *GateStatusPushInput) (*struct{}, error) {
	token := extractBearerToken(input.Authorization)
	if token == "" {
		return nil, huma.Error401Unauthorized("missing gate token")
	}

	gate, err := h.gates.AuthenticateToken(ctx, token)
	if err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			return nil, huma.Error401Unauthorized("invalid gate token")
		}
		return nil, huma.Error500InternalServerError("failed to authenticate gate", err)
	}

	status, meta, err := resolveInboundPayload(gate, input.Body)
	if err != nil {
		slog.Warn("inbound: payload mapping failed", "gate_id", gate.ID, "error", err)
		return nil, huma.Error400BadRequest("invalid payload: " + err.Error())
	}

	if err := h.gates.ProcessStatus(ctx, gate, status, meta); err != nil {
		return nil, huma.Error500InternalServerError("failed to update status", err)
	}

	slog.Info("inbound: gate status updated", "gate_id", gate.ID, "status", status)
	return nil, nil
}

func (h *GateInboundHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "gate-inbound-status",
		Method:      http.MethodPost,
		Path:        "/api/inbound/status",
		Summary:     "Gate reports its current status (authenticated by gate token)",
		Description: "Used by HTTP_INBOUND-mode gates to push their state to the server. " +
			"The raw JSON payload is interpreted according to the gate's status_config mapping " +
			"(status_config.type must be HTTP_INBOUND with a config.mapping). " +
			"Authentication: Authorization: Bearer {gate_token}.",
		Tags: []string{"Gate Inbound"},
	}, h.PushStatus)
}
