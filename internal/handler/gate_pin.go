package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	pins  *repository.GatePinRepository
	gates *repository.GateRepository
	mqtt  *internalmqtt.Client
	redis *redis.Client
}

func NewGatePinHandler(
	pins *repository.GatePinRepository,
	gates *repository.GateRepository,
	mqtt *internalmqtt.Client,
	redis *redis.Client,
) *GatePinHandler {
	return &GatePinHandler{pins: pins, gates: gates, mqtt: mqtt, redis: redis}
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

// --- Public: Unlock ---

type UnlockInput struct {
	Body struct {
		GateID uuid.UUID `json:"gate_id"`
		PIN    string    `json:"pin" minLength:"1"`
	}
}

// pinMetadata holds optional business rules stored in a gate pin's metadata field.
type pinMetadata struct {
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

func (h *GatePinHandler) Unlock(ctx context.Context, input *UnlockInput) (*struct{}, error) {
	// Pad response time to prevent timing-based attacks.
	start := time.Now()
	defer func() {
		if elapsed := time.Since(start); elapsed < minUnlockDuration {
			time.Sleep(minUnlockDuration - elapsed)
		}
	}()

	ip := middleware.ClientIPFromContext(ctx)

	// Rate limit: max 5 attempts per (gate + IP) in a 15-min fixed window.
	rateLimitKey := fmt.Sprintf("rl:unlock:%s:%s", input.Body.GateID, ip)
	pipe := h.redis.TxPipeline()
	incrCmd := pipe.Incr(ctx, rateLimitKey)
	pipe.ExpireNX(ctx, rateLimitKey, pinRateLimitTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("pin unlock: redis rate limit error, continuing without limiting", "error", err)
	} else if incrCmd.Val() > maxPINAttempts {
		return nil, huma.Error429TooManyRequests("too many attempts, try again later")
	}

	// Load all PINs for this gate.
	pins, err := h.pins.List(ctx, input.Body.GateID)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error")
	}

	// Ensure at least one bcrypt compare happens regardless of PIN count (constant time).
	if len(pins) == 0 {
		_ = bcrypt.CompareHashAndPassword(dummyPINHash, []byte(input.Body.PIN))
		return nil, huma.Error401Unauthorized("invalid pin")
	}

	// Find a matching PIN.
	var matchedPin *model.GatePin
	for _, p := range pins {
		if bcrypt.CompareHashAndPassword([]byte(p.HashedPin), []byte(input.Body.PIN)) == nil {
			matchedPin = p
			break
		}
	}
	if matchedPin == nil {
		return nil, huma.Error401Unauthorized("invalid pin")
	}

	// Check business rules (expiry, allowed days/hours).
	meta := parsePinMetadata(matchedPin.Metadata)
	if err := checkPINRules(meta, time.Now()); err != nil {
		slog.Info("pin unlock: business rule rejected", "pin_id", matchedPin.ID, "reason", err)
		return nil, huma.Error403Forbidden("access denied")
	}

	// Success — reset the rate limit counter.
	h.redis.Del(ctx, rateLimitKey)

	// Trigger open via integration driver.
	gate, gateErr := h.gates.GetByIDPublic(ctx, input.Body.GateID)
	if gateErr == nil {
		driver, driverErr := integration.NewOpenDriver(gate, h.mqtt)
		if driverErr != nil {
			slog.Warn("pin unlock: failed to build open driver", "gate_id", input.Body.GateID, "error", driverErr)
		} else if execErr := driver.Execute(ctx, gate); execErr != nil {
			slog.Warn("pin unlock: open driver failed", "gate_id", input.Body.GateID, "error", execErr)
		}
	}

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

	huma.Register(api, huma.Operation{
		OperationID: "public-unlock",
		Method:      http.MethodPost,
		Path:        "/api/public/unlock",
		Summary:     "Unlock a gate with a PIN code (no authentication required)",
		Tags:        []string{"Public"},
	}, h.Unlock)
}
