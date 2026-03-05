package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
)

// GateRepository is the data-access contract for gates.
// Implementations must be swappable (PostgreSQL, MongoDB, in-memory for tests, …).
// Business logic (e.g. status-rule evaluation) belongs in the caller, not here.
type GateRepository interface {
	// CRUD
	Create(ctx context.Context, wsID uuid.UUID, p CreateGateParams) (*model.Gate, error)
	GetByID(ctx context.Context, gateID, wsID uuid.UUID) (*model.Gate, error)
	GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error)
	GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*GatePublicInfo, error)
	Update(ctx context.Context, gateID, wsID uuid.UUID, p UpdateGateParams) (*model.Gate, error)
	Delete(ctx context.Context, gateID, wsID uuid.UUID) error
	ListForWorkspace(ctx context.Context, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID) ([]model.Gate, error)

	// Token management
	GetByToken(ctx context.Context, gateID uuid.UUID, token string) (*model.Gate, error)
	GetToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error)
	RotateToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error)

	// Status updates (pure data writes — no business logic)
	UpdateStatus(ctx context.Context, gateID uuid.UUID, status string, meta map[string]any) error

	// TTL
	MarkUnresponsiveWithIDs(ctx context.Context, ttl time.Duration) ([]UnresponsiveGate, error)
}
