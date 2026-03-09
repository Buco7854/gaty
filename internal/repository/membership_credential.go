package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// MembershipCredentialRepository is the data-access contract for workspace membership credentials.
type MembershipCredentialRepository interface {
	Create(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.MembershipCredential, error)
	GetByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) (*model.MembershipCredential, error)
	GetByID(ctx context.Context, credID, membershipID uuid.UUID) (*model.MembershipCredential, error)
	ListByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) ([]*model.MembershipCredential, error)
	FindBySSOIdentity(ctx context.Context, workspaceID uuid.UUID, providerSub string) (*model.MembershipCredential, error)
	// FindByHashedAPIToken looks up a valid (non-expired) API token by its SHA-256 hash and returns
	// the credential and its associated membership. Returns ErrNotFound if no match.
	FindByHashedAPIToken(ctx context.Context, hash string) (*model.MembershipCredential, *model.WorkspaceMembership, error)
	// UpdateHashedValue atomically replaces the hashed_value for a credential of the given type.
	UpdateHashedValue(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType, newHashedValue string) error
	Delete(ctx context.Context, credID, membershipID uuid.UUID) error
}
