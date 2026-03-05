package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type workspaceMembershipRepository struct {
	pool *pgxpool.Pool
}

func NewWorkspaceMembershipRepository(pool *pgxpool.Pool) repository.WorkspaceMembershipRepository {
	return &workspaceMembershipRepository{pool: pool}
}

const membershipColumns = `id, workspace_id, user_id, local_username, display_name, role, auth_config, invited_by, created_at`

func scanMembership(row pgx.Row) (*model.WorkspaceMembership, error) {
	m := &model.WorkspaceMembership{}
	err := row.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.LocalUsername, &m.DisplayName, &m.Role, &m.AuthConfig, &m.InvitedBy, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan membership: %w", err)
	}
	return m, nil
}

func (r *workspaceMembershipRepository) CreateLocal(ctx context.Context, workspaceID uuid.UUID, localUsername string, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO workspace_memberships (workspace_id, local_username, display_name, role, invited_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+membershipColumns,
		workspaceID, localUsername, displayName, role, invitedBy,
	)
	m, err := scanMembership(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("create local membership: %w", err)
	}
	return m, nil
}

func (r *workspaceMembershipRepository) CreateForUser(ctx context.Context, workspaceID, userID uuid.UUID, displayName *string, role model.WorkspaceRole, invitedBy *uuid.UUID) (*model.WorkspaceMembership, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO workspace_memberships (workspace_id, user_id, display_name, role, invited_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+membershipColumns,
		workspaceID, userID, displayName, role, invitedBy,
	)
	m, err := scanMembership(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("create user membership: %w", err)
	}
	return m, nil
}

func (r *workspaceMembershipRepository) GetByID(ctx context.Context, membershipID, workspaceID uuid.UUID) (*model.WorkspaceMembership, error) {
	return scanMembership(r.pool.QueryRow(ctx,
		`SELECT `+membershipColumns+` FROM workspace_memberships
		 WHERE id = $1 AND workspace_id = $2`,
		membershipID, workspaceID,
	))
}

func (r *workspaceMembershipRepository) GetByUserID(ctx context.Context, workspaceID, userID uuid.UUID) (*model.WorkspaceMembership, error) {
	return scanMembership(r.pool.QueryRow(ctx,
		`SELECT `+membershipColumns+` FROM workspace_memberships
		 WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	))
}

func (r *workspaceMembershipRepository) GetByLocalUsername(ctx context.Context, workspaceID uuid.UUID, localUsername string) (*model.WorkspaceMembership, error) {
	return scanMembership(r.pool.QueryRow(ctx,
		`SELECT `+membershipColumns+` FROM workspace_memberships
		 WHERE workspace_id = $1 AND local_username = $2`,
		workspaceID, localUsername,
	))
}

func (r *workspaceMembershipRepository) List(ctx context.Context, workspaceID uuid.UUID) ([]*model.WorkspaceMembership, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+membershipColumns+` FROM workspace_memberships
		 WHERE workspace_id = $1
		 ORDER BY created_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	defer rows.Close()

	var result []*model.WorkspaceMembership
	for rows.Next() {
		m := &model.WorkspaceMembership{}
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.LocalUsername, &m.DisplayName, &m.Role, &m.AuthConfig, &m.InvitedBy, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan membership row: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *workspaceMembershipRepository) Update(ctx context.Context, membershipID, workspaceID uuid.UUID, displayName *string, localUsername *string, role *model.WorkspaceRole, authConfig repository.Optional[map[string]any]) (*model.WorkspaceMembership, error) {
	sets := []string{}
	args := []any{membershipID, workspaceID}
	n := 3

	if displayName != nil {
		sets = append(sets, fmt.Sprintf("display_name = $%d", n))
		args = append(args, *displayName)
		n++
	}
	if localUsername != nil {
		sets = append(sets, fmt.Sprintf("local_username = $%d", n))
		args = append(args, *localUsername)
		n++
	}
	if role != nil {
		sets = append(sets, fmt.Sprintf("role = $%d", n))
		args = append(args, *role)
		n++
	}
	if authConfig.Set {
		sets = append(sets, fmt.Sprintf("auth_config = $%d::jsonb", n))
		if authConfig.V == nil {
			args = append(args, nil)
		} else {
			b, _ := json.Marshal(*authConfig.V)
			args = append(args, json.RawMessage(b))
		}
		n++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, membershipID, workspaceID)
	}

	row := r.pool.QueryRow(ctx,
		"UPDATE workspace_memberships SET "+strings.Join(sets, ", ")+" WHERE id = $1 AND workspace_id = $2 RETURNING "+membershipColumns,
		args...,
	)
	m, err := scanMembership(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("update membership: %w", err)
	}
	return m, nil
}

func (r *workspaceMembershipRepository) Delete(ctx context.Context, membershipID, workspaceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM workspace_memberships WHERE id = $1 AND workspace_id = $2`,
		membershipID, workspaceID,
	)
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *workspaceMembershipRepository) MergeUser(ctx context.Context, membershipID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspace_memberships
		 SET user_id = $2, local_username = NULL
		 WHERE id = $1 AND user_id IS NULL`,
		membershipID, userID,
	)
	if err != nil {
		return fmt.Errorf("merge user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
