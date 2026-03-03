package model

import "github.com/google/uuid"

type GatePolicy struct {
	GateID         uuid.UUID `json:"gate_id"`
	UserID         uuid.UUID `json:"user_id"`
	PermissionCode string    `json:"permission_code"`
}
