package repository

import (
	"context"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/google/uuid"
)

// WorkspaceMembershipRepository is the data-access contract for workspace memberships.
type WorkspaceMembershipRepository interface {
	CreateLocal(ctx context.Context, workspaceID uuid.UUID, localUsername string, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error)
	CreateForUser(ctx context.Context, workspaceID, userID uuid.UUID, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error)
	GetByID(ctx context.Context, membershipID, workspaceID uuid.UUID) (*model.WorkspaceMembership, error)
	GetByUserID(ctx context.Context, workspaceID, userID uuid.UUID) (*model.WorkspaceMembership, error)
	GetByLocalUsername(ctx context.Context, workspaceID uuid.UUID, localUsername string) (*model.WorkspaceMembership, error)
	List(ctx context.Context, workspaceID uuid.UUID, p model.PaginationParams) ([]*model.WorkspaceMembership, int, error)
	Update(ctx context.Context, membershipID, workspaceID uuid.UUID, displayName *string, localUsername *string, role *model.WorkspaceRole, authConfig OmittableNullable[map[string]any]) (*model.WorkspaceMembership, error)
	Delete(ctx context.Context, membershipID, workspaceID uuid.UUID) error
	MergeUser(ctx context.Context, membershipID, userID uuid.UUID) error
}
