package model

import (
	"time"

	"github.com/google/uuid"
)

type WorkspaceRole string

const (
	RoleOwner  WorkspaceRole = "OWNER"
	RoleAdmin  WorkspaceRole = "ADMIN"
	RoleMember WorkspaceRole = "MEMBER"
)

type Workspace struct {
	ID           uuid.UUID      `json:"id"`
	Name         string         `json:"name"`
	OwnerID      uuid.UUID      `json:"owner_id"`
	OIDCSettings map[string]any `json:"oidc_settings,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    *time.Time     `json:"-"`
}

type WorkspaceMember struct {
	WorkspaceID uuid.UUID     `json:"workspace_id"`
	UserID      uuid.UUID     `json:"user_id"`
	Role        WorkspaceRole `json:"role"`
}

type WorkspaceWithRole struct {
	Workspace
	Role WorkspaceRole `json:"role"`
}
