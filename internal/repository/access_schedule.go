package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// AccessScheduleRepository is the data-access contract for access schedules.
type AccessScheduleRepository interface {
	Create(ctx context.Context, workspaceID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error)
	GetByID(ctx context.Context, scheduleID, workspaceID uuid.UUID) (*model.AccessSchedule, error)
	GetByIDPublic(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error)
	List(ctx context.Context, workspaceID uuid.UUID) ([]*model.AccessSchedule, error)
	Update(ctx context.Context, scheduleID, workspaceID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error)
	Delete(ctx context.Context, scheduleID, workspaceID uuid.UUID) error
}
