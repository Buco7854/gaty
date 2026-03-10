package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// CreateGateParams holds all parameters for creating a new gate.
type CreateGateParams struct {
	Name              string
	IntegrationType   model.GateIntegrationType
	IntegrationConfig map[string]any
	OpenConfig        *model.ActionConfig
	CloseConfig       *model.ActionConfig
	StatusConfig      *model.ActionConfig
	MetaConfig        []model.MetaField
	StatusRules       []model.StatusRule
	CustomStatuses    []string
	TTLSeconds        *int
	StatusTransitions []model.StatusTransition
}

// UpdateGateParams holds the fields that can be updated on a gate.
// For action configs, Set=false = unchanged; Set=true && V=nil = clear to NULL.
// For slices, nil = unchanged, empty slice = clear.
type UpdateGateParams struct {
	Name              *string
	OpenConfig        OmittableNullable[model.ActionConfig]
	CloseConfig       OmittableNullable[model.ActionConfig]
	StatusConfig      OmittableNullable[model.ActionConfig]
	MetaConfig        []model.MetaField  // nil = unchanged, [] = clear
	StatusRules       []model.StatusRule
	CustomStatuses    []string // nil = unchanged, [] = clear
	TTLSeconds        OmittableNullable[int]
	StatusTransitions []model.StatusTransition // nil = unchanged, [] = clear
}

// GatePublicInfo holds gate + workspace context needed for the public PIN pad.
type GatePublicInfo struct {
	GateID         uuid.UUID
	GateName       string
	WorkspaceID    uuid.UUID
	WorkspaceName  string
	HasOpenAction  bool
	HasCloseAction bool
	Status         model.GateStatus
	MetaConfig     []model.MetaField
	StatusMetadata map[string]any
}

// UnresponsiveGate holds gate + workspace IDs for TTL expiry notifications.
type UnresponsiveGate struct {
	GateID      uuid.UUID
	WorkspaceID uuid.UUID
}

// GateRepository is the data-access contract for gates.
// Implementations must be swappable (PostgreSQL, MongoDB, in-memory for tests, …).
// Business logic (e.g. status-rule evaluation) belongs in the caller, not here.
type GateRepository interface {
	Create(ctx context.Context, wsID uuid.UUID, p CreateGateParams) (*model.Gate, error)
	GetByID(ctx context.Context, gateID, wsID uuid.UUID) (*model.Gate, error)
	GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error)
	GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*GatePublicInfo, error)
	Update(ctx context.Context, gateID, wsID uuid.UUID, p UpdateGateParams) (*model.Gate, error)
	Delete(ctx context.Context, gateID, wsID uuid.UUID) error
	ListForWorkspace(ctx context.Context, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID, p model.PaginationParams) ([]model.Gate, int, error)
	ListIDsForWorkspace(ctx context.Context, wsID uuid.UUID) ([]uuid.UUID, error)
	GetByToken(ctx context.Context, gateID uuid.UUID, token string) (*model.Gate, error)
	GetToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error)
	SetToken(ctx context.Context, gateID, wsID uuid.UUID, token string) error
	UpdateStatus(ctx context.Context, gateID uuid.UUID, status string, meta map[string]any) error
	MarkUnresponsiveWithIDs(ctx context.Context, ttl time.Duration) ([]UnresponsiveGate, error)
	// ListWebhookGates returns all gates configured with HTTP_WEBHOOK status polling.
	// Used by the webhook worker to determine which gates need periodic polling.
	ListWebhookGates(ctx context.Context) ([]model.Gate, error)
	// ListTransitionCandidates returns gates that have non-empty status_transitions
	// and a non-null last_seen_at. Timing checks are done in Go.
	ListTransitionCandidates(ctx context.Context) ([]model.Gate, error)
}
