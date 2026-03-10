package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type accessScheduleRepository struct {
	pool *pgxpool.Pool
}

func NewAccessScheduleRepository(pool *pgxpool.Pool) repository.AccessScheduleRepository {
	return &accessScheduleRepository{pool: pool}
}

const scheduleColumns = `id, workspace_id, membership_id, name, description, expr, created_at`

func scanSchedule(row pgx.Row) (*model.AccessSchedule, error) {
	s := &model.AccessSchedule{}
	var exprRaw []byte
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.MembershipID, &s.Name, &s.Description, &exprRaw, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan access schedule: %w", err)
	}
	if exprRaw != nil {
		var node model.ExprNode
		if err := json.Unmarshal(exprRaw, &node); err != nil {
			return nil, fmt.Errorf("unmarshal schedule expr: %w", err)
		}
		s.Expr = &node
	}
	return s, nil
}

func marshalExpr(expr *model.ExprNode) ([]byte, error) {
	if expr == nil {
		return nil, nil
	}
	b, err := json.Marshal(expr)
	if err != nil {
		return nil, fmt.Errorf("marshal expr: %w", err)
	}
	return b, nil
}

func (r *accessScheduleRepository) Create(ctx context.Context, workspaceID uuid.UUID, membershipID *uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error) {
	exprJSON, err := marshalExpr(expr)
	if err != nil {
		return nil, err
	}
	return scanSchedule(r.pool.QueryRow(ctx,
		`INSERT INTO access_schedules (workspace_id, membership_id, name, description, expr)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+scheduleColumns,
		workspaceID, membershipID, name, description, exprJSON,
	))
}

func (r *accessScheduleRepository) GetByID(ctx context.Context, scheduleID, workspaceID uuid.UUID) (*model.AccessSchedule, error) {
	return scanSchedule(r.pool.QueryRow(ctx,
		`SELECT `+scheduleColumns+` FROM access_schedules WHERE id = $1 AND workspace_id = $2`,
		scheduleID, workspaceID,
	))
}

func (r *accessScheduleRepository) GetByIDPublic(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error) {
	return scanSchedule(r.pool.QueryRow(ctx,
		`SELECT `+scheduleColumns+` FROM access_schedules WHERE id = $1`,
		scheduleID,
	))
}

func (r *accessScheduleRepository) List(ctx context.Context, workspaceID uuid.UUID, p model.PaginationParams) ([]*model.AccessSchedule, int, error) {
	p = p.Normalize()

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM access_schedules WHERE workspace_id = $1 AND membership_id IS NULL`,
		workspaceID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count schedules: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+scheduleColumns+` FROM access_schedules WHERE workspace_id = $1 AND membership_id IS NULL ORDER BY name LIMIT $2 OFFSET $3`,
		workspaceID, p.Limit, p.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	var result []*model.AccessSchedule
	for rows.Next() {
		s := &model.AccessSchedule{}
		var exprRaw []byte
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.MembershipID, &s.Name, &s.Description, &exprRaw, &s.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan schedule row: %w", err)
		}
		if exprRaw != nil {
			var node model.ExprNode
			if err := json.Unmarshal(exprRaw, &node); err != nil {
				return nil, 0, fmt.Errorf("unmarshal schedule expr: %w", err)
			}
			s.Expr = &node
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}

func (r *accessScheduleRepository) ListByMembership(ctx context.Context, membershipID, workspaceID uuid.UUID, p model.PaginationParams) ([]*model.AccessSchedule, int, error) {
	p = p.Normalize()

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM access_schedules WHERE membership_id = $1 AND workspace_id = $2`,
		membershipID, workspaceID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count member schedules: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+scheduleColumns+` FROM access_schedules WHERE membership_id = $1 AND workspace_id = $2 ORDER BY name LIMIT $3 OFFSET $4`,
		membershipID, workspaceID, p.Limit, p.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list member schedules: %w", err)
	}
	defer rows.Close()

	var result []*model.AccessSchedule
	for rows.Next() {
		s := &model.AccessSchedule{}
		var exprRaw []byte
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.MembershipID, &s.Name, &s.Description, &exprRaw, &s.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan schedule row: %w", err)
		}
		if exprRaw != nil {
			var node model.ExprNode
			if err := json.Unmarshal(exprRaw, &node); err != nil {
				return nil, 0, fmt.Errorf("unmarshal schedule expr: %w", err)
			}
			s.Expr = &node
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}

func (r *accessScheduleRepository) Update(ctx context.Context, scheduleID, workspaceID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error) {
	exprJSON, err := marshalExpr(expr)
	if err != nil {
		return nil, err
	}
	s, err := scanSchedule(r.pool.QueryRow(ctx,
		`UPDATE access_schedules
		 SET name = $3, description = $4, expr = $5
		 WHERE id = $1 AND workspace_id = $2
		 RETURNING `+scheduleColumns,
		scheduleID, workspaceID, name, description, exprJSON,
	))
	if err != nil {
		return nil, fmt.Errorf("update access schedule: %w", err)
	}
	return s, nil
}

func (r *accessScheduleRepository) Delete(ctx context.Context, scheduleID, workspaceID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM access_schedules WHERE id = $1 AND workspace_id = $2`,
		scheduleID, workspaceID,
	)
	if err != nil {
		return fmt.Errorf("delete access schedule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
