package model

import (
	"time"

	"github.com/google/uuid"
)

type WorkspaceMembership struct {
	ID            uuid.UUID      `json:"id"`
	WorkspaceID   uuid.UUID      `json:"workspace_id"`
	UserID        *uuid.UUID     `json:"user_id,omitempty"`
	LocalUsername *string        `json:"local_username,omitempty"`
	DisplayName   *string        `json:"display_name,omitempty"`
	Role          WorkspaceRole  `json:"role"`
	AuthConfig    map[string]any `json:"auth_config,omitempty"`
	InvitedBy     *uuid.UUID     `json:"invited_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}
