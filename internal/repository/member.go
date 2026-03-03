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

type MemberRepository struct {
	pool *pgxpool.Pool
}

func NewMemberRepository(pool *pgxpool.Pool) *MemberRepository {
	return &MemberRepository{pool: pool}
}

const memberColumns = `id, workspace_id, display_name, email, username, user_id, created_at`

func scanMember(row pgx.Row) (*model.Member, error) {
	m := &model.Member{}
	err := row.Scan(&m.ID, &m.WorkspaceID, &m.DisplayName, &m.Email, &m.Username, &m.UserID, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan member: %w", err)
	}
	return m, nil
}

func (r *MemberRepository) Create(ctx context.Context, workspaceID uuid.UUID, displayName string, email *string, username string) (*model.Member, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO members (workspace_id, display_name, email, username)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+memberColumns,
		workspaceID, displayName, email, username,
	)
	m, err := scanMember(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create member: %w", err)
	}
	return m, nil
}

func (r *MemberRepository) GetByID(ctx context.Context, memberID, workspaceID uuid.UUID) (*model.Member, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+memberColumns+`
		 FROM members
		 WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		memberID, workspaceID,
	)
	return scanMember(row)
}

// GetByUsernameOrEmail looks up a member by username OR email within a workspace.
// Used for member login.
func (r *MemberRepository) GetByUsernameOrEmail(ctx context.Context, workspaceID uuid.UUID, login string) (*model.Member, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+memberColumns+`
		 FROM members
		 WHERE workspace_id = $1
		   AND deleted_at IS NULL
		   AND (username = $2 OR email = $2)
		 LIMIT 1`,
		workspaceID, login,
	)
	return scanMember(row)
}

func (r *MemberRepository) List(ctx context.Context, workspaceID uuid.UUID) ([]*model.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+memberColumns+`
		 FROM members
		 WHERE workspace_id = $1 AND deleted_at IS NULL
		 ORDER BY display_name`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []*model.Member
	for rows.Next() {
		m := &model.Member{}
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.DisplayName, &m.Email, &m.Username, &m.UserID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member row: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *MemberRepository) Update(ctx context.Context, memberID, workspaceID uuid.UUID, displayName string, email *string) (*model.Member, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE members
		 SET display_name = $1, email = $2
		 WHERE id = $3 AND workspace_id = $4 AND deleted_at IS NULL
		 RETURNING `+memberColumns,
		displayName, email, memberID, workspaceID,
	)
	m, err := scanMember(row)
	if err != nil {
		return nil, fmt.Errorf("update member: %w", err)
	}
	return m, nil
}

func (r *MemberRepository) SoftDelete(ctx context.Context, memberID, workspaceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE members SET deleted_at = NOW()
		 WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		memberID, workspaceID,
	)
	if err != nil {
		return fmt.Errorf("delete member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

