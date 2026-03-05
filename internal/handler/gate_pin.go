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

	"github.com/Buco7854/gaty/internal/integration"
	"github.com/Buco7854/gaty/internal/middleware"
	"github.com/Buco7854/gaty/internal/model"
	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxPINAttempts    = 5
	pinRateLimitTTL   = 15 * time.Minute
	minUnlockDuration = 400 * time.Millisecond
)

// dummyPINHash is a pre-computed bcrypt hash used when a gate has no PINs,
// ensuring the unlock response time is consistent regardless of PIN count.
var dummyPINHash, _ = bcrypt.GenerateFromPassword([]byte("__dummy__"), bcrypt.DefaultCost)

type GatePinHandler struct {
	pins      *repository.GatePinRepository
	gates     *repository.GateRepository
	policies  *repository.PolicyRepository
	schedules *repository.AccessScheduleRepository
	mqtt      *internalmqtt.Client
	redis     *redis.Client
	authSvc   *service.AuthService
}

func NewGatePinHandler(
	pins *repository.GatePinRepository,
	gates *repository.GateRepository,
	policies *repository.PolicyRepository,
	schedules *repository.AccessScheduleRepository,
	mqtt *internalmqtt.Client,
	redis *redis.Client,
	authSvc *service.AuthService,
) *GatePinHandler {
	return &GatePinHandler{pins: pins, gates: gates, policies: policies, schedules: schedules, mqtt: mqtt, redis: redis, authSvc: authSvc}
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
	codeType := input.Body.CodeType
	if codeType == "" {
		codeType = "pin"
	}
	if codeType == "pin" {
		for _, ch := range input.Body.PIN {
			if ch < '0' || ch > '9' {
				return nil, huma.Error400BadRequest("PIN code must contain digits only")
			}
		}
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Body.PIN), bcrypt.DefaultCost)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash pin")
	}
	meta := input.Body.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	meta["code_type"] = codeType
	pin, err := h.pins.Create(ctx, input.GateID, string(hashed), input.Body.Label, meta, input.Body.ScheduleID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create pin")
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
		return nil, huma.Error500InternalServerError("failed to list pins")
	}
	if pins == nil {
		pins = []*model.GatePin{}
	}
	return &ListGatePinsOutput{Body: pins}, nil
}

// --- Admin: Update PIN ---

type UpdatePINInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	PinID       uuid.UUID `path:"pin_id"`
	Body        struct {
		Label    string         `json:"label" minLength:"1" maxLength:"100"`
		Metadata map[string]any `json:"metadata,omitempty"`
	}
}

func (h *GatePinHandler) UpdatePIN(ctx context.Context, input *UpdatePINInput) (*GatePinOutput, error) {
	pin, err := h.pins.Update(ctx, input.PinID, input.GateID, input.Body.Label, input.Body.Metadata)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update pin")
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
	pin, err := h.pins.SetPinSchedule(ctx, input.PinID, input.GateID, input.Body.ScheduleID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to set schedule")
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
	pin, err := h.pins.ClearPinSchedule(ctx, input.PinID, input.GateID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to clear schedule")
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
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("pin not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete pin")
	}
	// Clean up the use counter for this PIN.
	h.redis.Del(ctx, fmt.Sprintf("pin:uses:%s", input.PinID))
	return nil, nil
}

// --- PIN metadata ---

// pinMetadata holds optional business rules and session config stored in a gate pin's metadata.
type pinMetadata struct {
	// SessionDuration is the refresh-token TTL in seconds. 0 = infinite. Default: 7 days.
	// Controls how long the browser session remains valid after entering the PIN.
	SessionDuration *int64 `json:"session_duration"`

	// MaxUses is the maximum number of times this PIN can be used. 0 or absent = unlimited.
	MaxUses *int64 `json:"max_uses"`

	// Permissions lists gate permission codes granted to this PIN session.
	// Defaults to ["gate:trigger_open"] if empty. "gate:manage" is always excluded.
	Permissions []string `json:"permissions"`

	ExpiresAt         *time.Time `json:"expires_at"`
	AllowedDays       []int      `json:"allowed_days"`        // 0=Sun … 6=Sat
	AllowedHoursStart *int       `json:"allowed_hours_start"` // 0–23 inclusive
	AllowedHoursEnd   *int       `json:"allowed_hours_end"`   // 0–23 exclusive
}

func parsePinMetadata(raw map[string]any) pinMetadata {
	b, _ := json.Marshal(raw)
	var m pinMetadata
	_ = json.Unmarshal(b, &m)
	return m
}

func checkPINRules(meta pinMetadata, now time.Time) error {
	if meta.ExpiresAt != nil && now.After(*meta.ExpiresAt) {
		return fmt.Errorf("pin expired")
	}
	if len(meta.AllowedDays) > 0 {
		dow := int(now.Weekday())
		allowed := false
		for _, d := range meta.AllowedDays {
			if d == dow {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("not allowed today")
		}
	}
	if meta.AllowedHoursStart != nil && meta.AllowedHoursEnd != nil {
		h := now.Hour()
		if h < *meta.AllowedHoursStart || h >= *meta.AllowedHoursEnd {
			return fmt.Errorf("outside allowed hours")
		}
	}
	return nil
}

// validatePIN checks the rate limit, finds the matching PIN and validates its rules.
// Returns the matched pin and parsed metadata, or a Huma error.
func (h *GatePinHandler) validatePIN(ctx context.Context, gateID uuid.UUID, pin, ip string) (*model.GatePin, pinMetadata, error) {
	rateLimitKey := fmt.Sprintf("rl:unlock:%s:%s", gateID, ip)
	pipe := h.redis.TxPipeline()
	incrCmd := pipe.Incr(ctx, rateLimitKey)
	pipe.ExpireNX(ctx, rateLimitKey, pinRateLimitTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("pin validate: redis rate limit error, continuing without limiting", "error", err)
	} else if incrCmd.Val() > maxPINAttempts {
		return nil, pinMetadata{}, huma.Error429TooManyRequests("too many attempts, try again later")
	}

	pins, err := h.pins.List(ctx, gateID)
	if err != nil {
		return nil, pinMetadata{}, huma.Error500InternalServerError("internal error")
	}
	if len(pins) == 0 {
		_ = bcrypt.CompareHashAndPassword(dummyPINHash, []byte(pin))
		return nil, pinMetadata{}, huma.Error401Unauthorized("invalid pin")
	}

	var matched *model.GatePin
	for _, p := range pins {
		if bcrypt.CompareHashAndPassword([]byte(p.HashedPin), []byte(pin)) == nil {
			matched = p
			break
		}
	}
	if matched == nil {
		return nil, pinMetadata{}, huma.Error401Unauthorized("invalid pin")
	}

	meta := parsePinMetadata(matched.Metadata)
	now := time.Now()
	if err := checkPINRules(meta, now); err != nil {
		slog.Info("pin validate: business rule rejected", "pin_id", matched.ID, "reason", err)
		return nil, pinMetadata{}, huma.Error403Forbidden("access denied")
	}

	// Check attached access schedule if present.
	if matched.ScheduleID != nil {
		schedule, schErr := h.schedules.GetByIDPublic(ctx, *matched.ScheduleID)
		if schErr == nil {
			if schErr = CheckSchedule(schedule, now); schErr != nil {
				slog.Info("pin validate: schedule rejected", "pin_id", matched.ID, "schedule_id", matched.ScheduleID, "reason", schErr)
				return nil, pinMetadata{}, huma.Error403Forbidden("access denied")
			}
		}
	}

	// Reset rate limit on success.
	h.redis.Del(ctx, rateLimitKey)
	return matched, meta, nil
}

// triggerGate fires the gate's open driver, logging (not failing) any driver errors.
func (h *GatePinHandler) triggerGate(ctx context.Context, gateID uuid.UUID) {
	gate, err := h.gates.GetByIDPublic(ctx, gateID)
	if err != nil {
		return
	}
	driver, driverErr := integration.NewOpenDriver(gate, h.mqtt)
	if driverErr != nil {
		slog.Warn("gate trigger: failed to build open driver", "gate_id", gateID, "error", driverErr)
		return
	}
	if execErr := driver.Execute(ctx, gate); execErr != nil {
		slog.Warn("gate trigger: open driver failed", "gate_id", gateID, "error", execErr)
	}
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
		if elapsed := time.Since(start); elapsed < minUnlockDuration {
			time.Sleep(minUnlockDuration - elapsed)
		}
	}()

	ip := middleware.ClientIPFromContext(ctx)
	if _, _, err := h.validatePIN(ctx, input.Body.GateID, input.Body.PIN, ip); err != nil {
		return nil, err
	}
	h.triggerGate(ctx, input.Body.GateID)
	return nil, nil
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
		if elapsed := time.Since(start); elapsed < minUnlockDuration {
			time.Sleep(minUnlockDuration - elapsed)
		}
	}()

	ip := middleware.ClientIPFromContext(ctx)
	matched, meta, err := h.validatePIN(ctx, input.Body.GateID, input.Body.PIN, ip)
	if err != nil {
		return nil, err
	}

	// Check max_uses via atomic Redis increment (Lua ensures check-and-increment is atomic).
	if meta.MaxUses != nil && *meta.MaxUses > 0 {
		usesKey := fmt.Sprintf("pin:uses:%s", matched.ID)
		script := redis.NewScript(`
			local count = redis.call('INCR', KEYS[1])
			local max = tonumber(ARGV[1])
			if count > max then
				redis.call('DECR', KEYS[1])
				return 0
			end
			return count
		`)
		result, scriptErr := script.Run(ctx, h.redis, []string{usesKey}, *meta.MaxUses).Int64()
		if scriptErr != nil {
			slog.Warn("pin max_uses: redis script error, allowing use", "error", scriptErr)
		} else if result == 0 {
			return nil, huma.Error403Forbidden("pin max uses exceeded")
		}
	}

	// Always issue a session JWT. SessionDuration controls the refresh-token TTL.
	var sessionDuration time.Duration
	if meta.SessionDuration != nil {
		if *meta.SessionDuration == 0 {
			sessionDuration = 0 // infinite
		} else {
			sessionDuration = time.Duration(*meta.SessionDuration) * time.Second
		}
	} else {
		sessionDuration = 7 * 24 * time.Hour // default: 7 days
	}

	// Resolve permissions: use metadata if set, else default to trigger_open.
	perms := meta.Permissions
	if len(perms) == 0 {
		perms = []string{"gate:trigger_open"}
	}
	// Never grant gate:manage via PIN.
	filtered := make([]string, 0, len(perms))
	for _, p := range perms {
		if p != "gate:manage" {
			filtered = append(filtered, p)
		}
	}

	tokens, err := h.authSvc.IssueGatePinSession(ctx, matched.ID, input.Body.GateID, sessionDuration, filtered)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to issue session")
	}

	h.triggerGate(ctx, input.Body.GateID)
	out := &OpenGateOutput{}
	out.Body.Session = tokens
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
	_, gateID, permissions, err := h.authSvc.ValidatePinSessionToken(tokenStr)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired session")
	}

	action := input.Body.Action
	if action == "" {
		action = "open"
	}

	// Check that the PIN session grants the requested action.
	requiredPerm := "gate:trigger_open"
	if action == "close" {
		requiredPerm = "gate:trigger_close"
	}
	hasPermission := false
	for _, p := range permissions {
		if p == requiredPerm {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		return nil, huma.Error403Forbidden("missing " + requiredPerm + " permission")
	}

	gate, err := h.gates.GetByIDPublic(ctx, gateID)
	if err != nil {
		return nil, nil
	}
	if action == "close" {
		driver, driverErr := integration.NewCloseDriver(gate, h.mqtt)
		if driverErr == nil {
			_ = driver.Execute(ctx, gate)
		}
	} else {
		h.triggerGate(ctx, gateID)
	}
	return nil, nil
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
