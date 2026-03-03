package model

import "github.com/google/uuid"

type Permission struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type MembershipPolicy struct {
	MembershipID   uuid.UUID `json:"membership_id"`
	GateID         uuid.UUID `json:"gate_id"`
	PermissionCode string    `json:"permission_code"`
}
