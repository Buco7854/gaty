package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// GatePinRepository is the data-access contract for gate access codes (PINs).
type GatePinRepository interface {
	Create(ctx context.Context, gateID uuid.UUID, hashedPin string, label string, metadata map[string]any, scheduleID *uuid.UUID) (*model.GatePin, error)
	GetByID(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error)
	List(ctx context.Context, gateID uuid.UUID, p model.PaginationParams) ([]*model.GatePin, int, error)
	Update(ctx context.Context, pinID, gateID uuid.UUID, label *string, metadata map[string]any) (*model.GatePin, error)
	SetPinSchedule(ctx context.Context, pinID, gateID, scheduleID uuid.UUID) (*model.GatePin, error)
	ClearPinSchedule(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error)
	Delete(ctx context.Context, pinID, gateID uuid.UUID) error
}
