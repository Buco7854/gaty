package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type memberRepository struct {
	pool *pgxpool.Pool
}

func NewMemberRepository(pool *pgxpool.Pool) repository.MemberRepository {
	return &memberRepository{pool: pool}
}

const memberColumns = `id, username, display_name, role, created_at`

func scanMember(row pgx.Row) (*model.Member, error) {
	m := &model.Member{}
	err := row.Scan(&m.ID, &m.Username, &m.DisplayName, &m.Role, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan member: %w", err)
	}
	return m, nil
}

func (r *memberRepository) Create(ctx context.Context, username string, displayName *string, role model.Role) (*model.Member, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO members (username, display_name, role)
		 VALUES ($1, $2, $3)
		 RETURNING `+memberColumns,
		username, displayName, role,
	)
	m, err := scanMember(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("create member: %w", err)
	}
	return m, nil
}

func (r *memberRepository) GetByID(ctx context.Context, memberID uuid.UUID) (*model.Member, error) {
	return scanMember(r.pool.QueryRow(ctx,
		`SELECT `+memberColumns+` FROM members WHERE id = $1`,
		memberID,
	))
}

func (r *memberRepository) GetByUsername(ctx context.Context, username string) (*model.Member, error) {
	return scanMember(r.pool.QueryRow(ctx,
		`SELECT `+memberColumns+` FROM members WHERE username = $1`,
		username,
	))
}

func (r *memberRepository) HasAny(ctx context.Context) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM members LIMIT 1)`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check members: %w", err)
	}
	return exists, nil
}

func (r *memberRepository) List(ctx context.Context, p model.PaginationParams) ([]*model.Member, int, error) {
	p = p.Normalize()

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM members`,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count members: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+memberColumns+` FROM members
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		p.Limit, p.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var result []*model.Member
	for rows.Next() {
		m := &model.Member{}
		if err := rows.Scan(&m.ID, &m.Username, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan member row: %w", err)
		}
		result = append(result, m)
	}
	return result, total, rows.Err()
}

func (r *memberRepository) Update(ctx context.Context, memberID uuid.UUID, displayName *string, username *string, role *model.Role) (*model.Member, error) {
	sets := []string{}
	args := []any{memberID}
	n := 2

	if displayName != nil {
		sets = append(sets, fmt.Sprintf("display_name = $%d", n))
		args = append(args, *displayName)
		n++
	}
	if username != nil {
		sets = append(sets, fmt.Sprintf("username = $%d", n))
		args = append(args, *username)
		n++
	}
	if role != nil {
		sets = append(sets, fmt.Sprintf("role = $%d", n))
		args = append(args, *role)
		n++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, memberID)
	}

	row := r.pool.QueryRow(ctx,
		"UPDATE members SET "+strings.Join(sets, ", ")+" WHERE id = $1 RETURNING "+memberColumns,
		args...,
	)
	m, err := scanMember(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("update member: %w", err)
	}
	return m, nil
}

func (r *memberRepository) Delete(ctx context.Context, memberID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM members WHERE id = $1`,
		memberID,
	)
	if err != nil {
		return fmt.Errorf("delete member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
