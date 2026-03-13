package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// MemberRepository is the data-access contract for members.
type MemberRepository interface {
	Create(ctx context.Context, username string, displayName *string, role model.Role) (*model.Member, error)
	GetByID(ctx context.Context, memberID uuid.UUID) (*model.Member, error)
	GetByUsername(ctx context.Context, username string) (*model.Member, error)
	HasAny(ctx context.Context) (bool, error)
	List(ctx context.Context, p model.PaginationParams) ([]*model.Member, int, error)
	Update(ctx context.Context, memberID uuid.UUID, displayName *string, username *string, role *model.Role) (*model.Member, error)
	Delete(ctx context.Context, memberID uuid.UUID) error
}
