package model

import (
	"time"

	"github.com/google/uuid"
)

type CustomDomain struct {
	ID                uuid.UUID  `json:"id"`
	GateID            uuid.UUID  `json:"gate_id"`
	Domain            string     `json:"domain"`
	DNSChallengeToken string     `json:"dns_challenge_token"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func (d *CustomDomain) IsVerified() bool {
	return d.VerifiedAt != nil
}
