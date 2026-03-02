package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkspaceRepository struct {
	pool *pgxpool.Pool
}

func NewWorkspaceRepository(pool *pgxpool.Pool) *WorkspaceRepository {
	return &WorkspaceRepository{pool: pool}
}

func (r *WorkspaceRepository) Create(ctx context.Context, name string, ownerID uuid.UUID) (*model.Workspace, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	ws := &model.Workspace{}
	err = tx.QueryRow(ctx,
		`INSERT INTO workspaces (name, owner_id) VALUES ($1, $2)
		 RETURNING id, name, owner_id, created_at`,
		name, ownerID,
	).Scan(&ws.ID, &ws.Name, &ws.OwnerID, &ws.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, workspace_role) VALUES ($1, $2, 'OWNER')`,
		ws.ID, ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("add owner member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ws, nil
}

func (r *WorkspaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Workspace, error) {
	ws := &model.Workspace{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, owner_id, oidc_settings, created_at FROM workspaces
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&ws.ID, &ws.Name, &ws.OwnerID, &ws.OIDCSettings, &ws.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	return ws, nil
}

func (r *WorkspaceRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.WorkspaceWithRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT w.id, w.name, w.owner_id, w.created_at, wm.workspace_role
		 FROM workspaces w
		 JOIN workspace_members wm ON wm.workspace_id = w.id
		 WHERE wm.user_id = $1 AND w.deleted_at IS NULL
		 ORDER BY w.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var result []model.WorkspaceWithRole
	for rows.Next() {
		var wsr model.WorkspaceWithRole
		if err := rows.Scan(&wsr.ID, &wsr.Name, &wsr.OwnerID, &wsr.CreatedAt, &wsr.Role); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		result = append(result, wsr)
	}
	return result, rows.Err()
}

func (r *WorkspaceRepository) GetMemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (model.WorkspaceRole, error) {
	var role model.WorkspaceRole
	err := r.pool.QueryRow(ctx,
		`SELECT workspace_role FROM workspace_members
		 WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}
