package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// DomainResolveResult holds gate + workspace context for a verified custom domain.
type DomainResolveResult struct {
	GateID        uuid.UUID
	GateName      string
	WorkspaceID   uuid.UUID
	WorkspaceName string
}

// CustomDomainRepository is the data-access contract for custom domains.
type CustomDomainRepository interface {
	Create(ctx context.Context, gateID, workspaceID uuid.UUID, domain string) (*model.CustomDomain, error)
	GetByID(ctx context.Context, domainID, gateID uuid.UUID) (*model.CustomDomain, error)
	GetByDomain(ctx context.Context, domain string) (*model.CustomDomain, error)
	ListByGate(ctx context.Context, gateID uuid.UUID) ([]*model.CustomDomain, error)
	ListAllVerified(ctx context.Context) ([]*model.CustomDomain, error)
	SetVerified(ctx context.Context, domainID uuid.UUID, at time.Time) error
	ResolveByDomain(ctx context.Context, domain string) (*DomainResolveResult, error)
	Delete(ctx context.Context, domainID, gateID uuid.UUID) error
}
