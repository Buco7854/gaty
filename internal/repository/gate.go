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
type UpdateGateParams struct {
	Name              *string
	OpenConfig        OmittableNullable[model.ActionConfig]
	CloseConfig       OmittableNullable[model.ActionConfig]
	StatusConfig      OmittableNullable[model.ActionConfig]
	MetaConfig        []model.MetaField
	StatusRules       []model.StatusRule
	CustomStatuses    []string
	TTLSeconds        OmittableNullable[int]
	StatusTransitions []model.StatusTransition
}

// GatePublicInfo holds gate context needed for the public PIN pad.
type GatePublicInfo struct {
	GateID         uuid.UUID
	GateName       string
	HasOpenAction  bool
	HasCloseAction bool
	Status         model.GateStatus
	MetaConfig     []model.MetaField
	StatusMetadata map[string]any
}

// UnresponsiveGate holds gate IDs for TTL expiry notifications.
type UnresponsiveGate struct {
	GateID uuid.UUID
}

// GateRepository is the data-access contract for gates.
type GateRepository interface {
	Create(ctx context.Context, p CreateGateParams) (*model.Gate, error)
	GetByID(ctx context.Context, gateID uuid.UUID) (*model.Gate, error)
	GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error)
	GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*GatePublicInfo, error)
	Update(ctx context.Context, gateID uuid.UUID, p UpdateGateParams) (*model.Gate, error)
	Delete(ctx context.Context, gateID uuid.UUID) error
	List(ctx context.Context, role model.Role, memberID uuid.UUID, p model.PaginationParams) ([]model.Gate, int, error)
	ListIDs(ctx context.Context) ([]uuid.UUID, error)
	GetByToken(ctx context.Context, gateID uuid.UUID, token string) (*model.Gate, error)
	GetToken(ctx context.Context, gateID uuid.UUID) (string, error)
	SetToken(ctx context.Context, gateID uuid.UUID, token string) error
	UpdateStatus(ctx context.Context, gateID uuid.UUID, status string, meta map[string]any) error
	MarkUnresponsiveWithIDs(ctx context.Context, ttl time.Duration) ([]UnresponsiveGate, error)
	ListWebhookGates(ctx context.Context) ([]model.Gate, error)
	ListTransitionCandidates(ctx context.Context) ([]model.Gate, error)
}
