package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// AccessScheduleRepository is the data-access contract for access schedules.
type AccessScheduleRepository interface {
	Create(ctx context.Context, memberID *uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error)
	GetByID(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error)
	GetByIDPublic(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error)
	// List returns global schedules (member_id IS NULL).
	List(ctx context.Context, p model.PaginationParams) ([]*model.AccessSchedule, int, error)
	// ListByMember returns personal schedules belonging to a specific member.
	ListByMember(ctx context.Context, memberID uuid.UUID, p model.PaginationParams) ([]*model.AccessSchedule, int, error)
	Update(ctx context.Context, scheduleID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error)
	Delete(ctx context.Context, scheduleID uuid.UUID) error
}
