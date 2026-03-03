package model

import (
	"time"

	"github.com/google/uuid"
)

type Member struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	DisplayName string     `json:"display_name"`
	Email       *string    `json:"email,omitempty"`
	Username    string     `json:"username"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	DeletedAt   *time.Time `json:"-"`
}
