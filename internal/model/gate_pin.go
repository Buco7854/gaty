package model

import (
	"time"

	"github.com/google/uuid"
)

type GatePin struct {
	ID        uuid.UUID      `json:"id"`
	GateID    uuid.UUID      `json:"gate_id"`
	HashedPin string         `json:"-"`
	Label     string         `json:"label"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
