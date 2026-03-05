package repository

import (
	"context"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
)

// UserRepository is the data-access contract for platform users.
type UserRepository interface {
	Create(ctx context.Context, email string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	HasAny(ctx context.Context) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}
