package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// AccessScheduleRepository is the data-access contract for access schedules.
type AccessScheduleRepository interface {
	// Create creates a workspace-level schedule (membershipID == nil) or a member personal schedule.
	Create(ctx context.Context, workspaceID uuid.UUID, membershipID *uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error)
	GetByID(ctx context.Context, scheduleID, workspaceID uuid.UUID) (*model.AccessSchedule, error)
	GetByIDPublic(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error)
	// List returns workspace-level schedules (membership_id IS NULL).
	List(ctx context.Context, workspaceID uuid.UUID, p model.PaginationParams) ([]*model.AccessSchedule, int, error)
	// ListByMembership returns personal schedules belonging to a specific member.
	ListByMembership(ctx context.Context, membershipID, workspaceID uuid.UUID, p model.PaginationParams) ([]*model.AccessSchedule, int, error)
	Update(ctx context.Context, scheduleID, workspaceID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error)
	Delete(ctx context.Context, scheduleID, workspaceID uuid.UUID) error
}
