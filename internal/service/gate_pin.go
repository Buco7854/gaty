package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxPINAttempts     = 5
	maxGatePINAttempts = 50
	pinRateLimitTTL    = 15 * time.Minute

	// MinUnlockDuration is the minimum response time for unlock endpoints.
	// Enforced in the handler to prevent timing attacks on PIN validation.
	MinUnlockDuration = 400 * time.Millisecond
)

var (
	ErrInvalidPIN      = fmt.Errorf("invalid pin")
	ErrTooManyAttempts = fmt.Errorf("too many attempts")
	ErrMaxUsesExceeded = fmt.Errorf("pin max uses exceeded")
	ErrInvalidSession  = fmt.Errorf("invalid or expired session")
)

// dummyPINHash is a pre-computed bcrypt hash used when a gate has no PINs,
// ensuring consistent response time regardless of PIN count.
var dummyPINHash, _ = bcrypt.GenerateFromPassword([]byte("__dummy__"), bcrypt.DefaultCost)

// pinMetadata holds optional business rules stored in a gate pin's metadata.
type pinMetadata struct {
	SessionDuration *int64     `json:"session_duration"`
	MaxUses         *int64     `json:"max_uses"`
	Permissions     []string   `json:"permissions"`
	ExpiresAt       *time.Time `json:"expires_at"`
}

func parsePinMetadata(raw map[string]any) pinMetadata {
	b, _ := json.Marshal(raw)
	var m pinMetadata
	_ = json.Unmarshal(b, &m)
	return m
}

// CreatePINParams holds the fields for creating a new gate PIN.
type CreatePINParams struct {
	PIN        string
	CodeType   string // "pin" (digits only) or "password"
	Label      string
	Metadata   map[string]any
	ScheduleID *uuid.UUID
}

// UpdatePINParams holds the optional fields for updating a PIN.
type UpdatePINParams struct {
	Label    *string
	Metadata map[string]any
}

// OpenResult is returned by Open: tokens are nil for a one-shot PIN.
type OpenResult struct {
	Tokens *TokenPair
}

type GatePinService struct {
	pins      repository.GatePinRepository
	gates     repository.GateRepository
	schedules *ScheduleService
	auth      *AuthService
	trigger   GateTriggerFn
	redis     *redis.Client
}

func NewGatePinService(
	pins repository.GatePinRepository,
	gates repository.GateRepository,
	schedules *ScheduleService,
	auth *AuthService,
	trigger GateTriggerFn,
	redis *redis.Client,
) *GatePinService {
	return &GatePinService{
		pins:      pins,
		gates:     gates,
		schedules: schedules,
		auth:      auth,
		trigger:   trigger,
		redis:     redis,
	}
}

func (s *GatePinService) Create(ctx context.Context, gateID uuid.UUID, params CreatePINParams) (*model.GatePin, error) {
	codeType := params.CodeType
	if codeType == "" {
		codeType = "pin"
	}
	if codeType == "pin" {
		for _, ch := range params.PIN {
			if ch < '0' || ch > '9' {
				return nil, fmt.Errorf("pin code must contain digits only")
			}
		}
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(params.PIN), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash pin: %w", err)
	}
	meta := params.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	meta["code_type"] = codeType
	return s.pins.Create(ctx, gateID, string(hashed), params.Label, meta, params.ScheduleID)
}

func (s *GatePinService) List(ctx context.Context, gateID uuid.UUID) ([]*model.GatePin, error) {
	pins, err := s.pins.List(ctx, gateID)
	if err != nil {
		return nil, err
	}
	if pins == nil {
		pins = []*model.GatePin{}
	}
	return pins, nil
}

func (s *GatePinService) Update(ctx context.Context, pinID, gateID uuid.UUID, params UpdatePINParams) (*model.GatePin, error) {
	return s.pins.Update(ctx, pinID, gateID, params.Label, params.Metadata)
}

func (s *GatePinService) Delete(ctx context.Context, pinID, gateID uuid.UUID) error {
	if err := s.pins.Delete(ctx, pinID, gateID); err != nil {
		return err
	}
	s.redis.Del(ctx, fmt.Sprintf("pin:uses:%s", pinID))
	return nil
}

func (s *GatePinService) SetSchedule(ctx context.Context, pinID, gateID, scheduleID uuid.UUID) (*model.GatePin, error) {
	return s.pins.SetPinSchedule(ctx, pinID, gateID, scheduleID)
}

func (s *GatePinService) ClearSchedule(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error) {
	return s.pins.ClearPinSchedule(ctx, pinID, gateID)
}

// Unlock performs one-shot PIN validation and immediately triggers the gate open.
func (s *GatePinService) Unlock(ctx context.Context, gateID uuid.UUID, pin, ip string) error {
	if _, _, err := s.validatePIN(ctx, gateID, pin, ip); err != nil {
		return err
	}
	s.triggerGate(ctx, gateID)
	return nil
}

// Open validates the PIN, issues a session JWT, and triggers the gate.
func (s *GatePinService) Open(ctx context.Context, gateID uuid.UUID, pin, ip string) (*OpenResult, error) {
	matched, meta, err := s.validatePIN(ctx, gateID, pin, ip)
	if err != nil {
		return nil, err
	}

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
		result, scriptErr := script.Run(ctx, s.redis, []string{usesKey}, *meta.MaxUses).Int64()
		if scriptErr != nil {
			slog.Warn("pin max_uses: redis script error, allowing use", "error", scriptErr)
		} else if result == 0 {
			return nil, ErrMaxUsesExceeded
		}
	}

	var sessionDuration time.Duration
	if meta.SessionDuration != nil {
		sessionDuration = time.Duration(*meta.SessionDuration) * time.Second // 0 -> infinite
	} else {
		sessionDuration = 7 * 24 * time.Hour
	}

	perms := meta.Permissions
	if len(perms) == 0 {
		perms = []string{"gate:trigger_open"}
	}
	filtered := make([]string, 0, len(perms))
	for _, p := range perms {
		if p != "gate:manage" {
			filtered = append(filtered, p)
		}
	}

	tokens, err := s.auth.IssueGatePinSession(ctx, matched.ID, gateID, sessionDuration, filtered)
	if err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}

	s.triggerGate(ctx, gateID)
	return &OpenResult{Tokens: tokens}, nil
}

// TriggerWithSession validates a pin_session JWT and triggers the requested gate action.
func (s *GatePinService) TriggerWithSession(ctx context.Context, token, action string) error {
	_, gateID, permissions, err := s.auth.ValidatePinSessionToken(token)
	if err != nil {
		return ErrInvalidSession
	}

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
		return fmt.Errorf("%w: missing %s", model.ErrUnauthorized, requiredPerm)
	}

	gate, err := s.gates.GetByIDPublic(ctx, gateID)
	if err != nil {
		return err
	}

	s.trigger(ctx, gate, action)
	return nil
}

// validatePIN enforces rate limits, finds and bcrypt-verifies the PIN,
// checks expiry and any attached schedule.
func (s *GatePinService) validatePIN(ctx context.Context, gateID uuid.UUID, pin, ip string) (*model.GatePin, pinMetadata, error) {
	ipKey := fmt.Sprintf("rl:unlock:%s:%s", gateID, ip)
	gateKey := fmt.Sprintf("rl:unlock:gate:%s", gateID)
	pipe := s.redis.TxPipeline()
	incrIPCmd := pipe.Incr(ctx, ipKey)
	pipe.ExpireNX(ctx, ipKey, pinRateLimitTTL)
	incrGateCmd := pipe.Incr(ctx, gateKey)
	pipe.ExpireNX(ctx, gateKey, pinRateLimitTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Warn("pin validate: redis rate limit error, continuing without limiting", "error", err)
	} else if incrIPCmd.Val() > maxPINAttempts || incrGateCmd.Val() > maxGatePINAttempts {
		return nil, pinMetadata{}, ErrTooManyAttempts
	}

	pins, err := s.pins.List(ctx, gateID)
	if err != nil {
		return nil, pinMetadata{}, fmt.Errorf("list pins: %w", err)
	}
	if len(pins) == 0 {
		_ = bcrypt.CompareHashAndPassword(dummyPINHash, []byte(pin))
		return nil, pinMetadata{}, ErrInvalidPIN
	}

	var matched *model.GatePin
	for _, p := range pins {
		if bcrypt.CompareHashAndPassword([]byte(p.HashedPin), []byte(pin)) == nil {
			matched = p
			break
		}
	}
	if matched == nil {
		return nil, pinMetadata{}, ErrInvalidPIN
	}

	meta := parsePinMetadata(matched.Metadata)
	now := time.Now()
	if meta.ExpiresAt != nil && now.After(*meta.ExpiresAt) {
		return nil, pinMetadata{}, ErrScheduleDenied
	}
	if matched.ScheduleID != nil {
		if schedule, err := s.schedules.GetPublic(ctx, *matched.ScheduleID); err == nil {
			if err := s.schedules.Check(schedule, now); err != nil {
				slog.Info("pin validate: schedule rejected", "pin_id", matched.ID, "schedule_id", matched.ScheduleID)
				return nil, pinMetadata{}, ErrScheduleDenied
			}
		}
	}

	s.redis.Del(ctx, ipKey)
	return matched, meta, nil
}

func (s *GatePinService) triggerGate(ctx context.Context, gateID uuid.UUID) {
	gate, err := s.gates.GetByIDPublic(ctx, gateID)
	if err != nil {
		return
	}
	s.trigger(ctx, gate, "open")
}
