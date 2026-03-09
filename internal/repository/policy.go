package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// PolicyRepository is the data-access contract for membership policies.
type PolicyRepository interface {
	List(ctx context.Context, gateID uuid.UUID, p model.PaginationParams) ([]model.MembershipPolicy, int, error)
	ListForMembership(ctx context.Context, membershipID uuid.UUID, p model.PaginationParams) ([]model.MembershipPolicy, int, error)
	Grant(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error
	HasPermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) (bool, error)
	HasAnyPermission(ctx context.Context, membershipID, gateID uuid.UUID) (bool, error)
	Revoke(ctx context.Context, membershipID, gateID uuid.UUID) error
	RevokePermission(ctx context.Context, membershipID, gateID uuid.UUID, permCode string) error
	HasPermissionInWorkspace(ctx context.Context, membershipID uuid.UUID, workspaceID uuid.UUID, permCode string) (bool, error)
	SetMemberGateSchedule(ctx context.Context, membershipID, gateID, scheduleID uuid.UUID) error
	RemoveMemberGateSchedule(ctx context.Context, membershipID, gateID uuid.UUID) error
	GetMemberGateScheduleID(ctx context.Context, membershipID, gateID uuid.UUID) (uuid.UUID, error)
}
