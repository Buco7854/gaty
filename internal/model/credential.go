package model

import (
	"time"

	"github.com/google/uuid"
)

type CredentialType string

const (
	CredPassword    CredentialType = "PASSWORD"
	CredSSOIdentity CredentialType = "SSO_IDENTITY"
	CredAPIToken    CredentialType = "API_TOKEN"
)

type Credential struct {
	ID          uuid.UUID      `json:"id"`
	MemberID    uuid.UUID      `json:"member_id"`
	Type        CredentialType `json:"type"`
	HashedValue string         `json:"-"`
	Label       *string        `json:"label,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}
