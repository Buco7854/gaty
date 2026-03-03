package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Buco7854/gaty/internal/middleware"
	"github.com/Buco7854/gaty/internal/model"
	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type GateHandler struct {
	gates    *repository.GateRepository
	policies *repository.PolicyRepository
	audit    *repository.AuditRepository
	mqtt     *internalmqtt.Client // nil if broker unavailable
}

func NewGateHandler(
	gates *repository.GateRepository,
	policies *repository.PolicyRepository,
	audit *repository.AuditRepository,
	mqtt *internalmqtt.Client,
) *GateHandler {
	return &GateHandler{gates: gates, policies: policies, audit: audit, mqtt: mqtt}
}

// applyEffectiveStatus replaces Status with the computed live status.
func applyEffectiveStatus(g *model.Gate) {
	g.Status = g.EffectiveStatus()
}

// --- List gates ---

type ListGatesOutput struct {
	Body []model.Gate
}

func (h *GateHandler) List(ctx context.Context, input *WorkspacePathParam) (*ListGatesOutput, error) {
	userID, _ := middleware.UserIDFromContext(ctx)
	role, _ := middleware.WorkspaceRoleFromContext(ctx)

	gates, err := h.gates.ListForWorkspace(ctx, input.WorkspaceID, userID, role)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list gates")
	}
	if gates == nil {
		gates = []model.Gate{}
	}
	for i := range gates {
		applyEffectiveStatus(&gates[i])
	}
	return &ListGatesOutput{Body: gates}, nil
}

// --- Create gate ---

type CreateGateInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		Name              string                    `json:"name" minLength:"1"`
		IntegrationType   model.GateIntegrationType `json:"integration_type"`
		IntegrationConfig map[string]any            `json:"integration_config,omitempty"`
	}
}

type GateOutput struct {
	Body *model.Gate
}

func (h *GateHandler) Create(ctx context.Context, input *CreateGateInput) (*GateOutput, error) {
	gate, err := h.gates.Create(ctx, input.WorkspaceID, input.Body.Name, input.Body.IntegrationType, input.Body.IntegrationConfig)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create gate")
	}
	applyEffectiveStatus(gate)
	return &GateOutput{Body: gate}, nil
}

// --- Get gate ---

func (h *GateHandler) Get(ctx context.Context, input *GatePathParam) (*GateOutput, error) {
	role, _ := middleware.WorkspaceRoleFromContext(ctx)
	userID, _ := middleware.UserIDFromContext(ctx)

	gate, err := h.gates.GetByID(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate")
	}

	// MEMBER: verify they have at least one policy on this gate
	if role == model.RoleMember {
		ok, err := h.policies.HasAnyPermission(ctx, gate.ID, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to check permissions")
		}
		if !ok {
			return nil, huma.Error403Forbidden("access denied")
		}
	}

	applyEffectiveStatus(gate)
	return &GateOutput{Body: gate}, nil
}

// --- Update gate ---

type UpdateGateInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		Name              string         `json:"name" minLength:"1"`
		IntegrationConfig map[string]any `json:"integration_config,omitempty"`
	}
}

func (h *GateHandler) Update(ctx context.Context, input *UpdateGateInput) (*GateOutput, error) {
	gate, err := h.gates.Update(ctx, input.GateID, input.WorkspaceID, input.Body.Name, input.Body.IntegrationConfig)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update gate")
	}
	applyEffectiveStatus(gate)
	return &GateOutput{Body: gate}, nil
}

// --- Delete gate ---

type DeleteGateInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

func (h *GateHandler) Delete(ctx context.Context, input *DeleteGateInput) (*struct{}, error) {
	err := h.gates.SoftDelete(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete gate")
	}
	return nil, nil
}

// --- Trigger gate ---

type TriggerInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

func (h *GateHandler) Trigger(ctx context.Context, input *TriggerInput) (*struct{}, error) {
	role, _ := middleware.WorkspaceRoleFromContext(ctx)
	userID, _ := middleware.UserIDFromContext(ctx)

	// MEMBER: requires gate:trigger_open permission
	if role == model.RoleMember {
		ok, err := h.policies.HasPermission(ctx, input.GateID, userID, "gate:trigger_open")
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to check permissions")
		}
		if !ok {
			return nil, huma.Error403Forbidden("missing gate:trigger_open permission")
		}
	}

	// Verify gate exists in this workspace
	gate, err := h.gates.GetByID(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate")
	}

	// Publish MQTT command
	if h.mqtt != nil {
		payload, _ := json.Marshal(map[string]string{"action": "open"})
		topic := internalmqtt.CommandTopic(gate.WorkspaceID, gate.ID)
		if err := h.mqtt.Publish(topic, payload); err != nil {
			slog.Warn("mqtt: failed to publish command", "gate_id", gate.ID, "error", err)
		}
	}

	// Audit log (best-effort)
	if h.audit != nil {
		gateID := gate.ID
		uid := userID
		_ = h.audit.Insert(ctx, repository.AuditEntry{
			WorkspaceID: input.WorkspaceID,
			GateID:      &gateID,
			UserID:      &uid,
			Action:      "gate:trigger_open",
		})
	}

	return nil, nil
}

// RegisterRoutes wires all gate endpoints onto the Huma API.
func (h *GateHandler) RegisterRoutes(
	api huma.API,
	wsMember func(huma.Context, func(huma.Context)),
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID: "gate-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates",
		Summary:     "List gates for a workspace",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID:   "gate-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/gates",
		Summary:       "Create a gate",
		Tags:          []string{"Gates"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID: "gate-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}",
		Summary:     "Get a gate",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "gate-update",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}",
		Summary:     "Update a gate",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Update)

	huma.Register(api, huma.Operation{
		OperationID: "gate-delete",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}",
		Summary:     "Delete a gate",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Delete)

	huma.Register(api, huma.Operation{
		OperationID: "gate-trigger",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/trigger",
		Summary:     "Send open command to a gate",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.Trigger)
}
