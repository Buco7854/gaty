package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// CredentialRepository is the data-access contract for member credentials.
type CredentialRepository interface {
	Create(ctx context.Context, memberID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.Credential, error)
	GetByMemberAndType(ctx context.Context, memberID uuid.UUID, credType model.CredentialType) (*model.Credential, error)
	GetByID(ctx context.Context, credID, memberID uuid.UUID) (*model.Credential, error)
	ListByMemberAndType(ctx context.Context, memberID uuid.UUID, credType model.CredentialType, p model.PaginationParams) ([]*model.Credential, int, error)
	FindBySSOIdentity(ctx context.Context, providerSub string) (*model.Credential, error)
	// FindByHashedAPIToken looks up a valid (non-expired) API token by its SHA-256 hash.
	FindByHashedAPIToken(ctx context.Context, hash string) (*model.Credential, *model.Member, error)
	UpdateHashedValue(ctx context.Context, memberID uuid.UUID, credType model.CredentialType, newHashedValue string) error
	Delete(ctx context.Context, credID, memberID uuid.UUID) error
}
