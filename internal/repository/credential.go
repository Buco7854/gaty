package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// CredentialRepository is the data-access contract for platform user credentials.
type CredentialRepository interface {
	Create(ctx context.Context, userID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.Credential, error)
	GetByUserAndType(ctx context.Context, userID uuid.UUID, credType model.CredentialType) (*model.Credential, error)
	GetByID(ctx context.Context, credID uuid.UUID) (*model.Credential, error)
	ListByUserAndType(ctx context.Context, userID uuid.UUID, credType model.CredentialType) ([]*model.Credential, error)
	// UpdateHashedValue atomically replaces the hashed_value for a credential of the given type.
	UpdateHashedValue(ctx context.Context, userID uuid.UUID, credType model.CredentialType, newHashedValue string) error
	Delete(ctx context.Context, credID, userID uuid.UUID) error
}
