package model

import (
	"time"

	"github.com/google/uuid"
)

type MembershipCredential struct {
	ID           uuid.UUID      `json:"id"`
	MembershipID uuid.UUID      `json:"membership_id"`
	Type         CredentialType `json:"type"`
	HashedValue  string         `json:"-"`
	Label        *string        `json:"label,omitempty"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}
