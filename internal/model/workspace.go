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
	ID               uuid.UUID      `json:"id"`
	Name             string         `json:"name"`
	OwnerID          uuid.UUID      `json:"owner_id"`
	SSOSettings      map[string]any `json:"sso_settings,omitempty"`
	MemberAuthConfig map[string]any `json:"member_auth_config,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type WorkspaceWithRole struct {
	Workspace
	Role WorkspaceRole `json:"role"`
}
