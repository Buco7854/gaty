package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Buco7854/gaty/internal/integration"
	"github.com/Buco7854/gaty/internal/middleware"
	"github.com/Buco7854/gaty/internal/model"
	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type GateHandler struct {
	gates     repository.GateRepository
	policies  repository.PolicyRepository
	schedules repository.AccessScheduleRepository
	audit     repository.AuditRepository
	mqtt      *internalmqtt.Client // nil if broker unavailable
}

func NewGateHandler(
	gates repository.GateRepository,
	policies repository.PolicyRepository,
	schedules repository.AccessScheduleRepository,
	audit repository.AuditRepository,
	mqtt *internalmqtt.Client,
) *GateHandler {
	return &GateHandler{gates: gates, policies: policies, schedules: schedules, audit: audit, mqtt: mqtt}
}

// --- Shared path params (used by policy.go too) ---

type GatePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

func applyEffectiveStatus(g *model.Gate) {
	g.Status = g.EffectiveStatus()
}

// --- List gates ---

type ListGatesOutput struct {
	Body []model.Gate
}

func (h *GateHandler) List(ctx context.Context, input *WorkspacePathParam) (*ListGatesOutput, error) {
	role, _ := middleware.WorkspaceRoleFromContext(ctx)
	membershipID, _ := middleware.WorkspaceMembershipIDFromContext(ctx)

	gates, err := h.gates.ListForWorkspace(ctx, input.WorkspaceID, role, membershipID)
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
		IntegrationType   model.GateIntegrationType `json:"integration_type,omitempty"`
		IntegrationConfig map[string]any            `json:"integration_config,omitempty"`
		OpenConfig        *model.ActionConfig       `json:"open_config,omitempty"`
		CloseConfig       *model.ActionConfig       `json:"close_config,omitempty"`
		StatusConfig      *model.ActionConfig       `json:"status_config,omitempty"`
		// MetaConfig defines how status metadata fields are displayed.
		// Example: [{"key":"lora.snr","label":"SNR","unit":"dB"}]
		MetaConfig []model.MetaField `json:"meta_config,omitempty"`
		// StatusRules define conditions evaluated against metadata to override the gate status.
		// Example: [{"key":"battery","op":"lt","value":"20","set_status":"low_battery"}]
		StatusRules []model.StatusRule `json:"status_rules,omitempty"`
	}
}

type GateOutput struct {
	Body *model.Gate
}

func (h *GateHandler) Create(ctx context.Context, input *CreateGateInput) (*GateOutput, error) {
	intType := input.Body.IntegrationType
	if intType == "" {
		intType = model.IntegrationTypeMQTT
	}
	gate, err := h.gates.Create(ctx, input.WorkspaceID, repository.CreateGateParams{
		Name:              input.Body.Name,
		IntegrationType:   intType,
		IntegrationConfig: input.Body.IntegrationConfig,
		OpenConfig:        input.Body.OpenConfig,
		CloseConfig:       input.Body.CloseConfig,
		StatusConfig:      input.Body.StatusConfig,
		MetaConfig:        input.Body.MetaConfig,
		StatusRules:       input.Body.StatusRules,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create gate")
	}
	applyEffectiveStatus(gate)
	// GateToken is already populated by Create (returned once, on creation).
	return &GateOutput{Body: gate}, nil
}

// --- Get gate ---

func (h *GateHandler) Get(ctx context.Context, input *GatePathParam) (*GateOutput, error) {
	role, _ := middleware.WorkspaceRoleFromContext(ctx)

	gate, err := h.gates.GetByID(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate")
	}

	// MEMBER role: verify they have at least one policy on this gate.
	if role == model.RoleMember {
		membershipID, _ := middleware.WorkspaceMembershipIDFromContext(ctx)
		ok, err := h.policies.HasAnyPermission(ctx, membershipID, gate.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to check permissions")
		}
		if !ok {
			return nil, huma.Error403Forbidden("access denied")
		}
		// MEMBERs without gate:read_status don't see metadata.
		hasStatus, _ := h.policies.HasPermission(ctx, membershipID, gate.ID, "gate:read_status")
		if !hasStatus {
			gate.StatusMetadata = nil
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
		Name         string              `json:"name" minLength:"1"`
		OpenConfig   *model.ActionConfig `json:"open_config,omitempty"`
		CloseConfig  *model.ActionConfig `json:"close_config,omitempty"`
		StatusConfig *model.ActionConfig `json:"status_config,omitempty"`
		MetaConfig   []model.MetaField   `json:"meta_config,omitempty"`
		StatusRules  []model.StatusRule  `json:"status_rules,omitempty"`
	}
}

func (h *GateHandler) Update(ctx context.Context, input *UpdateGateInput) (*GateOutput, error) {
	gate, err := h.gates.Update(ctx, input.GateID, input.WorkspaceID, repository.UpdateGateParams{
		Name:         input.Body.Name,
		OpenConfig:   input.Body.OpenConfig,
		CloseConfig:  input.Body.CloseConfig,
		StatusConfig: input.Body.StatusConfig,
		MetaConfig:   input.Body.MetaConfig,
		StatusRules:  input.Body.StatusRules,
	})
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
	err := h.gates.Delete(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete gate")
	}
	return nil, nil
}

// --- Trigger gate (open or close) ---

type TriggerInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		// Action selects which gate action to perform. Defaults to "open".
		Action string `json:"action,omitempty" enum:"open,close" default:"open"`
	}
}

func (h *GateHandler) Trigger(ctx context.Context, input *TriggerInput) (*struct{}, error) {
	action := input.Body.Action
	if action == "" {
		action = "open"
	}

	role, _ := middleware.WorkspaceRoleFromContext(ctx)
	if role == model.RoleMember {
		membershipID, _ := middleware.WorkspaceMembershipIDFromContext(ctx)
		permCode := "gate:trigger_open"
		if action == "close" {
			permCode = "gate:trigger_close"
		}
		ok, err := h.policies.HasPermission(ctx, membershipID, input.GateID, permCode)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to check permissions")
		}
		if !ok {
			return nil, huma.Error403Forbidden("missing " + permCode + " permission")
		}
		// Check time-restriction schedule attached to this member-gate pair (if any).
		if scheduleID, schErr := h.policies.GetMemberGateScheduleID(ctx, membershipID, input.GateID); schErr == nil {
			schedule, schErr := h.schedules.GetByIDPublic(ctx, scheduleID)
			if schErr == nil {
				if schErr = CheckSchedule(schedule, time.Now()); schErr != nil {
					slog.Info("gate trigger: schedule rejected", "membership_id", membershipID, "gate_id", input.GateID, "schedule_id", scheduleID)
					return nil, huma.Error403Forbidden("access not allowed at this time")
				}
			}
		}
	}

	gate, err := h.gates.GetByID(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate")
	}

	var driver integration.Driver
	var driverErr error
	if action == "close" {
		driver, driverErr = integration.NewCloseDriver(gate, h.mqtt)
	} else {
		driver, driverErr = integration.NewOpenDriver(gate, h.mqtt)
	}
	if driverErr != nil {
		slog.Warn("gate: failed to build driver", "gate_id", gate.ID, "action", action, "error", driverErr)
	} else if execErr := driver.Execute(ctx, gate); execErr != nil {
		slog.Warn("gate: driver execution failed", "gate_id", gate.ID, "action", action, "error", execErr)
	}

	auditAction := "gate:trigger_open"
	if action == "close" {
		auditAction = "gate:trigger_close"
	}
	gateID := gate.ID
	_ = h.audit.Insert(ctx, repository.AuditEntry{
		WorkspaceID: input.WorkspaceID,
		GateID:      &gateID,
		Action:      auditAction,
	})

	return nil, nil
}

// --- Gate token management (admin only) ---

type GateTokenOutput struct {
	Body struct {
		GateID    uuid.UUID `json:"gate_id"`
		GateToken string    `json:"gate_token"`
	}
}

// GetToken returns the current gate authentication token.
func (h *GateHandler) GetToken(ctx context.Context, input *GatePathParam) (*GateTokenOutput, error) {
	token, err := h.gates.GetToken(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate token")
	}
	out := &GateTokenOutput{}
	out.Body.GateID = input.GateID
	out.Body.GateToken = token
	return out, nil
}

// RotateToken generates a new gate authentication token, invalidating the old one.
func (h *GateHandler) RotateToken(ctx context.Context, input *GatePathParam) (*GateTokenOutput, error) {
	token, err := h.gates.RotateToken(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to rotate gate token")
	}
	out := &GateTokenOutput{}
	out.Body.GateID = input.GateID
	out.Body.GateToken = token
	return out, nil
}

// RegisterRoutes wires all gate endpoints onto the Huma API.
func (h *GateHandler) RegisterRoutes(
	api huma.API,
	wsMember func(huma.Context, func(huma.Context)),
	wsAdmin func(huma.Context, func(huma.Context)),
	wsGateManager func(huma.Context, func(huma.Context)),
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
		Description:   "The response includes gate_token (the gate's auth secret). Store it — it won't be returned again (use rotate-token to get a new one).",
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
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
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
		Summary:     "Send open or close command to a gate",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.Trigger)

	// Token management: admin only.
	huma.Register(api, huma.Operation{
		OperationID: "gate-token-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/token",
		Summary:     "Get the gate authentication token",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.GetToken)

	huma.Register(api, huma.Operation{
		OperationID: "gate-token-rotate",
		Method:      http.MethodPost,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/token/rotate",
		Summary:     "Rotate (regenerate) the gate authentication token",
		Description: "Generates a new token and immediately invalidates the old one. Update the gate firmware.",
		Tags:        []string{"Gates"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.RotateToken)
}
