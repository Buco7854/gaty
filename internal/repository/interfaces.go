package repository

import (
	"context"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
)

// GateRepository is the data-access contract for gates.
// Implementations must be swappable (PostgreSQL, MongoDB, in-memory for tests, …).
// Business logic (e.g. status-rule evaluation) belongs in the caller, not here.
type GateRepository interface {
	// CRUD
	Create(ctx context.Context, wsID uuid.UUID, p CreateGateParams) (*model.Gate, error)
	GetByID(ctx context.Context, gateID, wsID uuid.UUID) (*model.Gate, error)
	GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error)
	GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*GatePublicInfo, error)
	Update(ctx context.Context, gateID, wsID uuid.UUID, p UpdateGateParams) (*model.Gate, error)
	Delete(ctx context.Context, gateID, wsID uuid.UUID) error
	ListForWorkspace(ctx context.Context, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID) ([]model.Gate, error)

	// Token management
	GetByToken(ctx context.Context, gateID uuid.UUID, token string) (*model.Gate, error)
	GetToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error)
	RotateToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error)

	// Status updates (pure data writes — no business logic)
	UpdateStatus(ctx context.Context, gateID uuid.UUID, status string, meta map[string]any) error

	// TTL
	MarkUnresponsiveWithIDs(ctx context.Context, ttl time.Duration) ([]UnresponsiveGate, error)
}

// GatePinRepository is the data-access contract for gate access codes (PINs).
type GatePinRepository interface {
	Create(ctx context.Context, gateID uuid.UUID, hashedPin string, label string, metadata map[string]any, scheduleID *uuid.UUID) (*model.GatePin, error)
	GetByID(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error)
	List(ctx context.Context, gateID uuid.UUID) ([]*model.GatePin, error)
	Update(ctx context.Context, pinID, gateID uuid.UUID, label string, metadata map[string]any) (*model.GatePin, error)
	SetPinSchedule(ctx context.Context, pinID, gateID, scheduleID uuid.UUID) (*model.GatePin, error)
	ClearPinSchedule(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error)
	Delete(ctx context.Context, pinID, gateID uuid.UUID) error
}

// CustomDomainRepository is the data-access contract for custom domains.
type CustomDomainRepository interface {
	Create(ctx context.Context, gateID, workspaceID uuid.UUID, domain string) (*model.CustomDomain, error)
	GetByID(ctx context.Context, domainID, gateID uuid.UUID) (*model.CustomDomain, error)
	GetByDomain(ctx context.Context, domain string) (*model.CustomDomain, error)
	ListByGate(ctx context.Context, gateID uuid.UUID) ([]*model.CustomDomain, error)
	ListAllVerified(ctx context.Context) ([]*model.CustomDomain, error)
	SetVerified(ctx context.Context, domainID uuid.UUID, at time.Time) error
	ResolveByDomain(ctx context.Context, domain string) (*DomainResolveResult, error)
	Delete(ctx context.Context, domainID, gateID uuid.UUID) error
}

// AccessScheduleRepository is the data-access contract for access schedules.
type AccessScheduleRepository interface {
	Create(ctx context.Context, workspaceID uuid.UUID, name string, description *string, rules []model.ScheduleRule) (*model.AccessSchedule, error)
	GetByID(ctx context.Context, scheduleID, workspaceID uuid.UUID) (*model.AccessSchedule, error)
	GetByIDPublic(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]*model.AccessSchedule, error)
	Update(ctx context.Context, scheduleID, workspaceID uuid.UUID, name string, description *string, rules []model.ScheduleRule) (*model.AccessSchedule, error)
	Delete(ctx context.Context, scheduleID, workspaceID uuid.UUID) error
}

// PolicyRepository is the data-access contract for membership policies.
type PolicyRepository interface {
	List(ctx context.Context, gateID uuid.UUID) ([]model.MembershipPolicy, error)
	ListForMembership(ctx context.Context, membershipID uuid.UUID) ([]model.MembershipPolicy, error)
	Grant(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error
	HasPermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) (bool, error)
	HasAnyPermission(ctx context.Context, membershipID, gateID uuid.UUID) (bool, error)
	Revoke(ctx context.Context, membershipID, gateID uuid.UUID) error
	RevokePermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error
	SetMemberGateSchedule(ctx context.Context, membershipID, gateID, scheduleID uuid.UUID) error
	RemoveMemberGateSchedule(ctx context.Context, membershipID, gateID uuid.UUID) error
	GetMemberGateScheduleID(ctx context.Context, membershipID, gateID uuid.UUID) (uuid.UUID, error)
}

// UserRepository is the data-access contract for platform users.
type UserRepository interface {
	Create(ctx context.Context, email string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	HasAny(ctx context.Context) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

// WorkspaceRepository is the data-access contract for workspaces.
type WorkspaceRepository interface {
	Create(ctx context.Context, name string, ownerID uuid.UUID) (*model.Workspace, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Workspace, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]model.WorkspaceWithRole, error)
	UpdateSSOSettings(ctx context.Context, id uuid.UUID, settings map[string]any) (*model.Workspace, error)
	UpdateMemberAuthConfig(ctx context.Context, id uuid.UUID, config map[string]any) (*model.Workspace, error)
	GetMemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (model.WorkspaceRole, error)
}

// WorkspaceMembershipRepository is the data-access contract for workspace memberships.
type WorkspaceMembershipRepository interface {
	CreateLocal(ctx context.Context, workspaceID uuid.UUID, localUsername string, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error)
	CreateForUser(ctx context.Context, workspaceID, userID uuid.UUID, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error)
	GetByID(ctx context.Context, membershipID, workspaceID uuid.UUID) (*model.WorkspaceMembership, error)
	GetByUserID(ctx context.Context, workspaceID, userID uuid.UUID) (*model.WorkspaceMembership, error)
	GetByLocalUsername(ctx context.Context, workspaceID uuid.UUID, localUsername string) (*model.WorkspaceMembership, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]*model.WorkspaceMembership, error)
	Update(ctx context.Context, membershipID, workspaceID uuid.UUID, displayName *string, localUsername *string, role *model.WorkspaceRole, authConfig map[string]any) (*model.WorkspaceMembership, error)
	Delete(ctx context.Context, membershipID, workspaceID uuid.UUID) error
	MergeUser(ctx context.Context, membershipID, userID uuid.UUID) error
}

// CredentialRepository is the data-access contract for platform user credentials.
type CredentialRepository interface {
	Create(ctx context.Context, userID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.Credential, error)
	GetByUserAndType(ctx context.Context, userID uuid.UUID, credType model.CredentialType) (*model.Credential, error)
	GetByID(ctx context.Context, credID uuid.UUID) (*model.Credential, error)
	ListByUserAndType(ctx context.Context, userID uuid.UUID, credType model.CredentialType) ([]*model.Credential, error)
	Delete(ctx context.Context, credID, userID uuid.UUID) error
}

// MembershipCredentialRepository is the data-access contract for workspace membership credentials.
type MembershipCredentialRepository interface {
	Create(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.MembershipCredential, error)
	GetByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) (*model.MembershipCredential, error)
	GetByID(ctx context.Context, credID, membershipID uuid.UUID) (*model.MembershipCredential, error)
	ListByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) ([]*model.MembershipCredential, error)
	FindBySSOIdentity(ctx context.Context, workspaceID uuid.UUID, providerSub string) (*model.MembershipCredential, error)
	Delete(ctx context.Context, credID, membershipID uuid.UUID) error
}

// AuditRepository is the data-access contract for audit log entries.
type AuditRepository interface {
	Insert(ctx context.Context, entry AuditEntry) error
}
