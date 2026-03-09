package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// WorkspaceRepository is the data-access contract for workspaces.
type WorkspaceRepository interface {
	Create(ctx context.Context, name string, ownerID uuid.UUID) (*model.Workspace, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Workspace, error)
	ListForUser(ctx context.Context, userID uuid.UUID, p model.PaginationParams) ([]model.WorkspaceWithRole, int, error)
	Rename(ctx context.Context, id uuid.UUID, name string) (*model.Workspace, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateSSOSettings(ctx context.Context, id uuid.UUID, settings map[string]any) (*model.Workspace, error)
	UpdateMemberAuthConfig(ctx context.Context, id uuid.UUID, config map[string]any) (*model.Workspace, error)
	GetMemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (model.WorkspaceRole, error)
}
