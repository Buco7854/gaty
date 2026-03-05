package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
)

// MembershipCredentialRepository is the data-access contract for workspace membership credentials.
type MembershipCredentialRepository interface {
	Create(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.MembershipCredential, error)
	GetByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) (*model.MembershipCredential, error)
	GetByID(ctx context.Context, credID, membershipID uuid.UUID) (*model.MembershipCredential, error)
	ListByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) ([]*model.MembershipCredential, error)
	FindBySSOIdentity(ctx context.Context, workspaceID uuid.UUID, providerSub string) (*model.MembershipCredential, error)
	Delete(ctx context.Context, credID, membershipID uuid.UUID) error
}
