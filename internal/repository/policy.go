package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// PolicyRepository is the data-access contract for member access policies.
type PolicyRepository interface {
	Grant(ctx context.Context, memberID, gateID uuid.UUID, permCode string) error
	Revoke(ctx context.Context, memberID, gateID uuid.UUID) error
	RevokePermission(ctx context.Context, memberID, gateID uuid.UUID, permCode string) error
	HasPermission(ctx context.Context, memberID, gateID uuid.UUID, permCode string) (bool, error)
	HasAnyPermission(ctx context.Context, memberID, gateID uuid.UUID) (bool, error)
	HasPermissionOnAnyGate(ctx context.Context, memberID uuid.UUID, permCode string) (bool, error)
	ListForGate(ctx context.Context, gateID uuid.UUID, p model.PaginationParams) ([]model.MemberPolicy, int, error)
	ListForMember(ctx context.Context, memberID uuid.UUID, p model.PaginationParams) ([]model.MemberPolicy, int, error)
	GetScheduleID(ctx context.Context, memberID, gateID uuid.UUID) (uuid.UUID, error)
	SetSchedule(ctx context.Context, memberID, gateID, scheduleID uuid.UUID) error
	RemoveSchedule(ctx context.Context, memberID, gateID uuid.UUID) error
}
