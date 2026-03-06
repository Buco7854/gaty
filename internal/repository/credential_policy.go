package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// CredentialPolicyRepository manages access policies and schedule links for API credentials.
// Uses access_policies (subject_type='credential') and schedule_links (subject_type='credential', gate_id IS NULL).
type CredentialPolicyRepository interface {
	List(ctx context.Context, credentialID uuid.UUID) ([]model.CredentialPolicy, error)
	HasAny(ctx context.Context, credentialID uuid.UUID) (bool, error)
	HasPermission(ctx context.Context, credentialID, gateID uuid.UUID, permCode string) (bool, error)
	Grant(ctx context.Context, credentialID, gateID uuid.UUID, permCode string) error
	RevokeAll(ctx context.Context, credentialID uuid.UUID) error
	GetScheduleID(ctx context.Context, credentialID uuid.UUID) (uuid.UUID, error)
	SetSchedule(ctx context.Context, credentialID, scheduleID uuid.UUID) error
	RemoveSchedule(ctx context.Context, credentialID uuid.UUID) error
}
