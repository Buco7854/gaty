package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type GatePinHandler struct {
	pins *service.GatePinService
}

func NewGatePinHandler(pins *service.GatePinService) *GatePinHandler {
	return &GatePinHandler{pins: pins}
}

// --- Admin: Create PIN ---

type CreatePINInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		PIN        string         `json:"pin" minLength:"1"`
		CodeType   string         `json:"code_type,omitempty" enum:"pin,password" default:"pin"`
		Label      string         `json:"label" minLength:"1" maxLength:"100"`
		Metadata   map[string]any `json:"metadata,omitempty"`
		ScheduleID *uuid.UUID     `json:"schedule_id,omitempty"`
	}
}

type GatePinOutput struct {
	Body *model.GatePin
}

func (h *GatePinHandler) CreatePIN(ctx context.Context, input *CreatePINInput) (*GatePinOutput, error) {
	pin, err := h.pins.Create(ctx, input.GateID, service.CreatePINParams{
		PIN:        input.Body.PIN,
		CodeType:   input.Body.CodeType,
		Label:      input.Body.Label,
		Metadata:   input.Body.Metadata,
		ScheduleID: input.Body.ScheduleID,
	})
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &GatePinOutput{Body: pin}, nil
}

// --- Admin: List PINs ---

type ListGatePinsOutput struct {
	Body []*model.GatePin
}

func (h *GatePinHandler) ListPINs(ctx context.Context, input *GatePathParam) (*ListGatePinsOutput, error) {
	pins, err := h.pins.List(ctx, input.GateID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list pins", err)
	}
	return &ListGatePinsOutput{Body: pins}, nil
}

// --- Admin: Update PIN ---

type UpdatePINInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	PinID       uuid.UUID `path:"pin_id"`
	Body        struct {
		Label    *string        `json:"label,omitempty" minLength:"1" maxLength:"100"`
		Metadata map[string]any `json:"metadata,omitempty"`
	}
}

func (h *GatePinHandler) UpdatePIN(ctx context.Context, input *UpdatePINInput) (*GatePinOutput, error) {
	pin, err := h.pins.Update(ctx, input.PinID, input.GateID, service.UpdatePINParams{
		Label:    input.Body.Label,
		Metadata: input.Body.Metadata,
	})
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update pin", err)
	}
	return &GatePinOutput{Body: pin}, nil
}

// --- Admin: Set schedule on a PIN ---

type SetPinScheduleInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	PinID       uuid.UUID `path:"pin_id"`
	Body        struct {
		ScheduleID uuid.UUID `json:"schedule_id"`
	}
}

func (h *GatePinHandler) SetPinSchedule(ctx context.Context, input *SetPinScheduleInput) (*GatePinOutput, error) {
	pin, err := h.pins.SetSchedule(ctx, input.PinID, input.GateID, input.Body.ScheduleID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to set schedule", err)
	}
	return &GatePinOutput{Body: pin}, nil
}

// --- Admin: Clear schedule from a PIN ---

type PinIDPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	PinID       uuid.UUID `path:"pin_id"`
}

func (h *GatePinHandler) ClearPinSchedule(ctx context.Context, input *PinIDPathParam) (*GatePinOutput, error) {
	pin, err := h.pins.ClearSchedule(ctx, input.PinID, input.GateID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to clear schedule", err)
	}
	return &GatePinOutput{Body: pin}, nil
}

// --- Admin: Delete PIN ---

type DeletePINInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	PinID       uuid.UUID `path:"pin_id"`
}

func (h *GatePinHandler) DeletePIN(ctx context.Context, input *DeletePINInput) (*struct{}, error) {
	err := h.pins.Delete(ctx, input.PinID, input.GateID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete pin", err)
	}
	return nil, nil
}

// --- Public: Unlock (one-shot, backward-compatible) ---

type UnlockInput struct {
	Body struct {
		GateID uuid.UUID `json:"gate_id"`
		PIN    string    `json:"pin" minLength:"1"`
	}
}

func (h *GatePinHandler) Unlock(ctx context.Context, input *UnlockInput) (*struct{}, error) {
	start := time.Now()
	defer func() {
		if elapsed := time.Since(start); elapsed < service.MinUnlockDuration {
			time.Sleep(service.MinUnlockDuration - elapsed)
		}
	}()

	ip := middleware.ClientIPFromContext(ctx)
	return nil, mapPINError(h.pins.Unlock(ctx, input.Body.GateID, input.Body.PIN, ip))
}

// --- Public: Open (smart: one-shot or session based on PIN metadata) ---

type OpenGateInput struct {
	Body struct {
		GateID uuid.UUID `json:"gate_id"`
		PIN    string    `json:"pin" minLength:"1"`
	}
}

type OpenGateOutput struct {
	Body struct {
		// Session is nil for one-shot PINs; populated for session PINs.
		Session *service.TokenPair `json:"session,omitempty"`
	}
}

func (h *GatePinHandler) OpenGate(ctx context.Context, input *OpenGateInput) (*OpenGateOutput, error) {
	start := time.Now()
	defer func() {
		if elapsed := time.Since(start); elapsed < service.MinUnlockDuration {
			time.Sleep(service.MinUnlockDuration - elapsed)
		}
	}()

	ip := middleware.ClientIPFromContext(ctx)
	result, err := h.pins.Open(ctx, input.Body.GateID, input.Body.PIN, ip)
	if err != nil {
		return nil, mapPINError(err)
	}
	out := &OpenGateOutput{}
	out.Body.Session = result.Tokens
	return out, nil
}

// --- Public: Trigger (use stored pin_session JWT) ---

type PublicTriggerInput struct {
	Authorization string `header:"Authorization" required:"true"`
	Body          struct {
		Action string `json:"action,omitempty" enum:"open,close" default:"open"`
	}
}

func (h *GatePinHandler) PublicTrigger(ctx context.Context, input *PublicTriggerInput) (*struct{}, error) {
	tokenStr := strings.TrimPrefix(input.Authorization, "Bearer ")
	action := input.Body.Action
	if action == "" {
		action = "open"
	}

	err := h.pins.TriggerWithSession(ctx, tokenStr, action)
	if errors.Is(err, service.ErrInvalidSession) {
		return nil, huma.Error401Unauthorized("invalid or expired session")
	}
	if errors.Is(err, model.ErrUnauthorized) {
		requiredPerm := "gate:trigger_open"
		if action == "close" {
			requiredPerm = "gate:trigger_close"
		}
		return nil, huma.Error403Forbidden("missing " + requiredPerm + " permission")
	}
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("gate not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to trigger gate", err)
	}
	return nil, nil
}

// mapPINError converts service-level PIN errors to Huma HTTP errors.
func mapPINError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, service.ErrTooManyAttempts) {
		return huma.Error429TooManyRequests("too many attempts, try again later")
	}
	if errors.Is(err, service.ErrInvalidPIN) {
		return huma.Error401Unauthorized("invalid pin")
	}
	if errors.Is(err, service.ErrScheduleDenied) {
		return huma.Error403Forbidden("access denied")
	}
	if errors.Is(err, service.ErrMaxUsesExceeded) {
		return huma.Error403Forbidden("pin max uses exceeded")
	}
	return huma.Error500InternalServerError("internal error", err)
}

// RegisterRoutes wires gate pin endpoints onto the Huma API.
func (h *GatePinHandler) RegisterRoutes(
	api huma.API,
	wsMember func(huma.Context, func(huma.Context)),
	wsGateManager func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID:   "gate-pin-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/gates/{gate_id}/pins",
		Summary:       "Create a PIN code for a gate",
		Tags:          []string{"Gate Pins"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsMember, wsGateManager},
	}, h.CreatePIN)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins",
		Summary:     "List PIN codes for a gate",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.ListPINs)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-update",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins/{pin_id}",
		Summary:     "Update an access code (label, metadata)",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.UpdatePIN)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-delete",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins/{pin_id}",
		Summary:     "Delete an access code",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.DeletePIN)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-set-schedule",
		Method:      http.MethodPut,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins/{pin_id}/schedule",
		Summary:     "Attach (or replace) a time-restriction schedule on a PIN",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.SetPinSchedule)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-clear-schedule",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins/{pin_id}/schedule",
		Summary:     "Remove the time-restriction schedule from a PIN",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsMember, wsGateManager},
	}, h.ClearPinSchedule)

	// Backward-compatible one-shot unlock (always opens immediately, no session).
	huma.Register(api, huma.Operation{
		OperationID: "public-unlock",
		Method:      http.MethodPost,
		Path:        "/api/public/unlock",
		Summary:     "Unlock a gate with a PIN code (no authentication required)",
		Tags:        []string{"Public"},
	}, h.Unlock)

	// Smart open: creates a session if the PIN is type=session, triggers gate in all cases.
	huma.Register(api, huma.Operation{
		OperationID: "public-open",
		Method:      http.MethodPost,
		Path:        "/api/public/open",
		Summary:     "Open a gate with a PIN code; returns a session JWT if the PIN is session-type",
		Tags:        []string{"Public"},
	}, h.OpenGate)

	// Trigger a gate using a stored pin_session JWT (no PIN re-entry needed).
	huma.Register(api, huma.Operation{
		OperationID: "public-trigger",
		Method:      http.MethodPost,
		Path:        "/api/public/trigger",
		Summary:     "Trigger gate open using an active pin session JWT",
		Tags:        []string{"Public"},
	}, h.PublicTrigger)
}
