package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type workspaceRepository struct {
	pool *pgxpool.Pool
}

func NewWorkspaceRepository(pool *pgxpool.Pool) repository.WorkspaceRepository {
	return &workspaceRepository{pool: pool}
}

const workspaceColumns = `id, name, owner_id, sso_settings, member_auth_config, created_at`

func scanWorkspace(row pgx.Row) (*model.Workspace, error) {
	ws := &model.Workspace{}
	err := row.Scan(&ws.ID, &ws.Name, &ws.OwnerID, &ws.SSOSettings, &ws.MemberAuthConfig, &ws.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan workspace: %w", err)
	}
	return ws, nil
}

func (r *workspaceRepository) Create(ctx context.Context, name string, ownerID uuid.UUID) (*model.Workspace, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	ws, err := scanWorkspace(tx.QueryRow(ctx,
		`INSERT INTO workspaces (name, owner_id) VALUES ($1, $2)
		 RETURNING `+workspaceColumns,
		name, ownerID,
	))
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO workspace_memberships (workspace_id, user_id, role) VALUES ($1, $2, 'OWNER')`,
		ws.ID, ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("add owner membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ws, nil
}

func (r *workspaceRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Workspace, error) {
	return scanWorkspace(r.pool.QueryRow(ctx,
		`SELECT `+workspaceColumns+` FROM workspaces WHERE id = $1`,
		id,
	))
}

func (r *workspaceRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.WorkspaceWithRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT w.id, w.name, w.owner_id, w.sso_settings, w.member_auth_config, w.created_at, wm.role
		 FROM workspaces w
		 JOIN workspace_memberships wm ON wm.workspace_id = w.id
		 WHERE wm.user_id = $1
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
		if err := rows.Scan(&wsr.ID, &wsr.Name, &wsr.OwnerID, &wsr.SSOSettings, &wsr.MemberAuthConfig, &wsr.CreatedAt, &wsr.Role); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		result = append(result, wsr)
	}
	return result, rows.Err()
}

func (r *workspaceRepository) UpdateSSOSettings(ctx context.Context, id uuid.UUID, settings map[string]any) (*model.Workspace, error) {
	if settings == nil {
		settings = map[string]any{}
	}
	ws, err := scanWorkspace(r.pool.QueryRow(ctx,
		`UPDATE workspaces SET sso_settings = $2 WHERE id = $1 RETURNING `+workspaceColumns,
		id, settings,
	))
	if err != nil {
		return nil, fmt.Errorf("update sso settings: %w", err)
	}
	return ws, nil
}

func (r *workspaceRepository) UpdateMemberAuthConfig(ctx context.Context, id uuid.UUID, config map[string]any) (*model.Workspace, error) {
	if config == nil {
		config = map[string]any{}
	}
	ws, err := scanWorkspace(r.pool.QueryRow(ctx,
		`UPDATE workspaces SET member_auth_config = $2 WHERE id = $1 RETURNING `+workspaceColumns,
		id, config,
	))
	if err != nil {
		return nil, fmt.Errorf("update member auth config: %w", err)
	}
	return ws, nil
}

func (r *workspaceRepository) GetMemberRole(ctx context.Context, workspaceID, userID uuid.UUID) (model.WorkspaceRole, error) {
	var role model.WorkspaceRole
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM workspace_memberships WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", repository.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}
