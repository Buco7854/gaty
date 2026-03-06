package model

import "github.com/google/uuid"

type CredentialPolicy struct {
	CredentialID   uuid.UUID `json:"credential_id"`
	GateID         uuid.UUID `json:"gate_id"`
	PermissionCode string    `json:"permission_code"`
}
