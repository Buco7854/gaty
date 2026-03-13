package model

import "github.com/google/uuid"

type Permission struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type MemberPolicy struct {
	MemberID       uuid.UUID `json:"member_id"`
	GateID         uuid.UUID `json:"gate_id"`
	PermissionCode string    `json:"permission_code"`
}
