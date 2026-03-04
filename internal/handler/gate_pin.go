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
	pins    *repository.GatePinRepository
	gates   *repository.GateRepository
	mqtt    *internalmqtt.Client
	redis   *redis.Client
	authSvc *service.AuthService
}

func NewGatePinHandler(
	pins *repository.GatePinRepository,
	gates *repository.GateRepository,
	mqtt *internalmqtt.Client,
	redis *redis.Client,
	authSvc *service.AuthService,
) *GatePinHandler {
	return &GatePinHandler{pins: pins, gates: gates, mqtt: mqtt, redis: redis, authSvc: authSvc}
}

// --- Admin: Create PIN ---

type CreatePINInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	GateID      uuid.UUID `path:"gate_id"`
	Body        struct {
		PIN      string         `json:"pin" minLength:"4" maxLength:"20"`
		Label    *string        `json:"label,omitempty" maxLength:"100"`
		Metadata map[string]any `json:"metadata,omitempty"`
	}
}

type GatePinOutput struct {
	Body *model.GatePin
}

func (h *GatePinHandler) CreatePIN(ctx context.Context, input *CreatePINInput) (*GatePinOutput, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Body.PIN), bcrypt.DefaultCost)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to hash pin")
	}
	pin, err := h.pins.Create(ctx, input.GateID, string(hashed), input.Body.Label, input.Body.Metadata)
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
	return nil, nil
}

// --- PIN metadata ---

// pinMetadata holds optional business rules and session config stored in a gate pin's metadata.
type pinMetadata struct {
	// Type controls unlock behaviour: "one_shot" (default) opens immediately with no session;
	// "session" issues a JWT that allows repeated opens without re-entering the PIN.
	Type string `json:"type"` // "one_shot" | "session"

	// SessionDuration is the refresh-token TTL in seconds (session type only). 0 = infinite.
	SessionDuration *int64 `json:"session_duration"`

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
	if err := checkPINRules(meta, time.Now()); err != nil {
		slog.Info("pin validate: business rule rejected", "pin_id", matched.ID, "reason", err)
		return nil, pinMetadata{}, huma.Error403Forbidden("access denied")
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

	out := &OpenGateOutput{}

	if meta.Type == "session" {
		var sessionDuration time.Duration
		if meta.SessionDuration != nil {
			if *meta.SessionDuration == 0 {
				sessionDuration = 0 // infinite
			} else {
				sessionDuration = time.Duration(*meta.SessionDuration) * time.Second
			}
		} else {
			sessionDuration = 7 * 24 * time.Hour // default
		}

		tokens, err := h.authSvc.IssueGatePinSession(ctx, matched.ID, input.Body.GateID, sessionDuration)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to issue session")
		}
		out.Body.Session = tokens
	}

	h.triggerGate(ctx, input.Body.GateID)
	return out, nil
}

// --- Public: Trigger (use stored pin_session JWT) ---

type PublicTriggerInput struct {
	Authorization string `header:"Authorization" required:"true"`
}

func (h *GatePinHandler) PublicTrigger(ctx context.Context, input *PublicTriggerInput) (*struct{}, error) {
	tokenStr := strings.TrimPrefix(input.Authorization, "Bearer ")
	_, gateID, err := h.authSvc.ValidatePinSessionToken(tokenStr)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired session")
	}
	h.triggerGate(ctx, gateID)
	return nil, nil
}

// RegisterRoutes wires gate pin endpoints onto the Huma API.
func (h *GatePinHandler) RegisterRoutes(
	api huma.API,
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID:   "gate-pin-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/gates/{gate_id}/pins",
		Summary:       "Create a PIN code for a gate",
		Tags:          []string{"Gate Pins"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.CreatePIN)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins",
		Summary:     "List PIN codes for a gate",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.ListPINs)

	huma.Register(api, huma.Operation{
		OperationID: "gate-pin-delete",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/gates/{gate_id}/pins/{pin_id}",
		Summary:     "Delete a PIN code",
		Tags:        []string{"Gate Pins"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.DeletePIN)

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
