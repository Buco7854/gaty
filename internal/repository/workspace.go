package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// AddMember adds a user to a workspace with the given role.
// Returns ErrAlreadyExists if the user is already a member, ErrNotFound if the user doesn't exist.
func (r *WorkspaceRepository) AddMember(ctx context.Context, workspaceID, userID uuid.UUID, role model.WorkspaceRole) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, workspace_role) VALUES ($1, $2, $3)`,
		workspaceID, userID, role,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return ErrAlreadyExists
			}
			if pgErr.Code == "23503" { // foreign_key_violation
				return ErrNotFound
			}
		}
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// UpdateMemberRole changes the role of an existing member.
// Returns ErrNotFound if the user is not a member.
func (r *WorkspaceRepository) UpdateMemberRole(ctx context.Context, workspaceID, userID uuid.UUID, role model.WorkspaceRole) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspace_members SET workspace_role = $3
		 WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID, role,
	)
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveMember removes a user from a workspace.
// Returns ErrNotFound if the user is not a member.
func (r *WorkspaceRepository) RemoveMember(ctx context.Context, workspaceID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
