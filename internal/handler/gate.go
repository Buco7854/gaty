package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type GateHandler struct {
	gates *service.GateService
}

func NewGateHandler(gates *service.GateService) *GateHandler {
	return &GateHandler{gates: gates}
}

// --- Shared path params (used by policy.go too) ---

type GatePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

// --- List gates ---

type ListGatesOutput struct {
	Body []model.Gate
}

func (h *GateHandler) List(ctx context.Context, input *WorkspacePathParam) (*ListGatesOutput, error) {
	role, _ := middleware.WorkspaceRoleFromContext(ctx)
	membershipID, _ := middleware.WorkspaceMembershipIDFromContext(ctx)

	gates, err := h.gates.List(ctx, input.WorkspaceID, role, membershipID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list gates", err)
	}
	return &ListGatesOutput{Body: gates}, nil
}

// --- Create gate ---

type CreateGateInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		Name              string                    `json:"name" minLength:"1"`
		IntegrationType   model.GateIntegrationType `json:"integration_type,omitempty" default:"MQTT"`
		IntegrationConfig map[string]any            `json:"integration_config,omitempty"`
		OpenConfig        *model.ActionConfig       `json:"open_config,omitempty"`
		CloseConfig       *model.ActionConfig       `json:"close_config,omitempty"`
		StatusConfig      *model.ActionConfig       `json:"status_config,omitempty"`
		// MetaConfig defines how status metadata fields are displayed.
		MetaConfig []model.MetaField `json:"meta_config,omitempty"`
		// StatusRules define conditions evaluated against metadata to override the gate status.
		StatusRules []model.StatusRule `json:"status_rules,omitempty"`
		// CustomStatuses are user-defined statuses in addition to the defaults (open, closed, unavailable).
		CustomStatuses    []string                    `json:"custom_statuses,omitempty"`
		TTLSeconds        *int                        `json:"ttl_seconds,omitempty"`
		StatusTransitions []model.StatusTransition    `json:"status_transitions,omitempty"`
	}
}

type GateOutput struct {
	Body *model.Gate
}

func gateFieldErr(field string, err error) error {
	return huma.Error400BadRequest("validation failed", &huma.ErrorDetail{
		Message:  err.Error(),
		Location: "/body/" + field,
	})
}

func (h *GateHandler) Create(ctx context.Context, input *CreateGateInput) (*GateOutput, error) {
	b := input.Body
	if err := validateActionConfig("open_config", b.OpenConfig); err != nil {
		return nil, gateFieldErr("open_config", err)
	}
	if err := validateActionConfig("close_config", b.CloseConfig); err != nil {
		return nil, gateFieldErr("close_config", err)
	}
	if err := validateStatusActionConfig(b.StatusConfig); err != nil {
		return nil, gateFieldErr("status_config", err)
	}
	if err := validateCustomStatuses(b.CustomStatuses); err != nil {
		return nil, gateFieldErr("custom_statuses", err)
	}
	if err := validateStatusRules(b.StatusRules, b.CustomStatuses); err != nil {
		return nil, gateFieldErr("status_rules", err)
	}
	if err := validateTTLSeconds(b.TTLSeconds); err != nil {
		return nil, gateFieldErr("ttl_seconds", err)
	}
	if err := validateStatusTransitions(b.StatusTransitions, b.CustomStatuses); err != nil {
		return nil, gateFieldErr("status_transitions", err)
	}
	gate, err := h.gates.Create(ctx, input.WorkspaceID, service.CreateGateParams{
		Name:              input.Body.Name,
		IntegrationType:   input.Body.IntegrationType,
		IntegrationConfig: input.Body.IntegrationConfig,
		OpenConfig:        input.Body.OpenConfig,
		CloseConfig:       input.Body.CloseConfig,
		StatusConfig:      input.Body.StatusConfig,
		MetaConfig:        input.Body.MetaConfig,
		StatusRules:       input.Body.StatusRules,
		CustomStatuses:    input.Body.CustomStatuses,
		TTLSeconds:        input.Body.TTLSeconds,
		StatusTransitions: input.Body.StatusTransitions,
	})
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create gate", err)
	}
	// GateToken is already populated by Create (returned once, on creation).
	return &GateOutput{Body: gate}, nil
}

// --- Get gate ---

func (h *GateHandler) Get(ctx context.Context, input *GatePathParam) (*GateOutput, error) {
	role, _ := middleware.WorkspaceRoleFromContext(ctx)
	membershipID, _ := middleware.WorkspaceMembershipIDFromContext(ctx)

	gate, err := h.gates.Get(ctx, input.GateID, input.WorkspaceID, role, membershipID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if errors.Is(err, model.ErrUnauthorized) {
		return nil, huma.Error403Forbidden("access denied")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate", err)
	}
	return &GateOutput{Body: gate}, nil
}

// --- Update gate ---

type UpdateGateInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		// All fields are optional: omit a field to leave it unchanged.
		// Action configs: null = clear to NULL in DB, omit = unchanged.
		// For meta_config, status_rules, custom_statuses: send an empty array [] to clear.
		Name           *string                              `json:"name,omitempty" minLength:"1"`
		OpenConfig     OmittableNullable[model.ActionConfig] `json:"open_config,omitempty"`
		CloseConfig    OmittableNullable[model.ActionConfig] `json:"close_config,omitempty"`
		StatusConfig   OmittableNullable[model.ActionConfig] `json:"status_config,omitempty"`
		MetaConfig     []model.MetaField                    `json:"meta_config,omitempty"`
		StatusRules    []model.StatusRule                    `json:"status_rules,omitempty"`
		CustomStatuses []string                             `json:"custom_statuses,omitempty"`
		TTLSeconds        OmittableNullable[int]               `json:"ttl_seconds,omitempty"`
		StatusTransitions []model.StatusTransition             `json:"status_transitions,omitempty"`
	}
}

func (h *GateHandler) Update(ctx context.Context, input *UpdateGateInput) (*GateOutput, error) {
	b := input.Body
	if b.OpenConfig.Sent && !b.OpenConfig.Null {
		if err := validateActionConfig("open_config", &b.OpenConfig.Value); err != nil {
			return nil, gateFieldErr("open_config", err)
		}
	}
	if b.CloseConfig.Sent && !b.CloseConfig.Null {
		if err := validateActionConfig("close_config", &b.CloseConfig.Value); err != nil {
			return nil, gateFieldErr("close_config", err)
		}
	}
	if b.StatusConfig.Sent && !b.StatusConfig.Null {
		if err := validateStatusActionConfig(&b.StatusConfig.Value); err != nil {
			return nil, gateFieldErr("status_config", err)
		}
	}
	if b.CustomStatuses != nil {
		if err := validateCustomStatuses(b.CustomStatuses); err != nil {
			return nil, gateFieldErr("custom_statuses", err)
		}
	}
	if b.StatusRules != nil {
		if err := validateStatusRules(b.StatusRules, b.CustomStatuses); err != nil {
			return nil, gateFieldErr("status_rules", err)
		}
	}
	if b.TTLSeconds.Sent && !b.TTLSeconds.Null {
		if err := validateTTLSeconds(&b.TTLSeconds.Value); err != nil {
			return nil, gateFieldErr("ttl_seconds", err)
		}
	}
	if b.StatusTransitions != nil {
		if err := validateStatusTransitions(b.StatusTransitions, b.CustomStatuses); err != nil {
			return nil, gateFieldErr("status_transitions", err)
		}
	}
	gate, err := h.gates.Update(ctx, input.GateID, input.WorkspaceID, service.UpdateGateParams{
		Name:              input.Body.Name,
		OpenConfig:        input.Body.OpenConfig.ToModel(),
		CloseConfig:       input.Body.CloseConfig.ToModel(),
		StatusConfig:      input.Body.StatusConfig.ToModel(),
		MetaConfig:        input.Body.MetaConfig,
		StatusRules:       input.Body.StatusRules,
		CustomStatuses:    input.Body.CustomStatuses,
		TTLSeconds:        input.Body.TTLSeconds.ToModel(),
		StatusTransitions: input.Body.StatusTransitions,
	})
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update gate", err)
	}
	return &GateOutput{Body: gate}, nil
}

// --- Delete gate ---

type DeleteGateInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
}

func (h *GateHandler) Delete(ctx context.Context, input *DeleteGateInput) (*struct{}, error) {
	err := h.gates.Delete(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete gate", err)
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
	membershipID, _ := middleware.WorkspaceMembershipIDFromContext(ctx)
	credentialID, _ := middleware.CredentialIDFromContext(ctx)

	err := h.gates.Trigger(ctx, input.GateID, input.WorkspaceID, role, membershipID, credentialID, action)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if errors.Is(err, model.ErrUnauthorized) {
		permCode := "gate:trigger_open"
		if action == "close" {
			permCode = "gate:trigger_close"
		}
		return nil, huma.Error403Forbidden("missing " + permCode + " permission")
	}
	if errors.Is(err, service.ErrScheduleDenied) {
		return nil, huma.Error403Forbidden("access not allowed at this time")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to trigger gate", err)
	}
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
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get gate token", err)
	}
	out := &GateTokenOutput{}
	out.Body.GateID = input.GateID
	out.Body.GateToken = token
	return out, nil
}

// RotateToken generates a new gate authentication token, invalidating the old one.
func (h *GateHandler) RotateToken(ctx context.Context, input *GatePathParam) (*GateTokenOutput, error) {
	token, err := h.gates.RotateToken(ctx, input.GateID, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to rotate gate token", err)
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
