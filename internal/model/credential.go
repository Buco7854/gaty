package model

import (
	"time"

	"github.com/google/uuid"
)

type CredentialTargetType string
type CredentialType string

const (
	TargetUser   CredentialTargetType = "USER"
	TargetMember CredentialTargetType = "MEMBER"
	TargetGate   CredentialTargetType = "GATE"

	CredPassword     CredentialType = "PASSWORD"
	CredPINCode      CredentialType = "PIN_CODE"
	CredAPIToken     CredentialType = "API_TOKEN"
	CredOIDCIdentity CredentialType = "OIDC_IDENTITY"
)

type Credential struct {
	ID             uuid.UUID            `json:"id"`
	TargetType     CredentialTargetType `json:"target_type"`
	TargetID       uuid.UUID            `json:"target_id"`
	CredentialType CredentialType       `json:"credential_type"`
	HashedValue    string               `json:"-"`
	Metadata       map[string]any       `json:"metadata,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}
