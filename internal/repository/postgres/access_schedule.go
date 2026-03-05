package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
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

const scheduleColumns = `id, workspace_id, name, description, rules, created_at`

func scanSchedule(row pgx.Row) (*model.AccessSchedule, error) {
	s := &model.AccessSchedule{}
	var rulesRaw []byte
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.Name, &s.Description, &rulesRaw, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan access schedule: %w", err)
	}
	if err := json.Unmarshal(rulesRaw, &s.Rules); err != nil {
		return nil, fmt.Errorf("unmarshal schedule rules: %w", err)
	}
	if s.Rules == nil {
		s.Rules = []model.ScheduleRule{}
	}
	return s, nil
}

func (r *accessScheduleRepository) Create(ctx context.Context, workspaceID uuid.UUID, name string, description *string, rules []model.ScheduleRule) (*model.AccessSchedule, error) {
	if rules == nil {
		rules = []model.ScheduleRule{}
	}
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return nil, fmt.Errorf("marshal rules: %w", err)
	}
	return scanSchedule(r.pool.QueryRow(ctx,
		`INSERT INTO access_schedules (workspace_id, name, description, rules)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+scheduleColumns,
		workspaceID, name, description, rulesJSON,
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

func (r *accessScheduleRepository) List(ctx context.Context, workspaceID uuid.UUID) ([]*model.AccessSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+scheduleColumns+` FROM access_schedules WHERE workspace_id = $1 ORDER BY name`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	var result []*model.AccessSchedule
	for rows.Next() {
		s := &model.AccessSchedule{}
		var rulesRaw []byte
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.Name, &s.Description, &rulesRaw, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan schedule row: %w", err)
		}
		if err := json.Unmarshal(rulesRaw, &s.Rules); err != nil {
			return nil, fmt.Errorf("unmarshal schedule rules: %w", err)
		}
		if s.Rules == nil {
			s.Rules = []model.ScheduleRule{}
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *accessScheduleRepository) Update(ctx context.Context, scheduleID, workspaceID uuid.UUID, name string, description *string, rules []model.ScheduleRule) (*model.AccessSchedule, error) {
	if rules == nil {
		rules = []model.ScheduleRule{}
	}
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return nil, fmt.Errorf("marshal rules: %w", err)
	}
	s, err := scanSchedule(r.pool.QueryRow(ctx,
		`UPDATE access_schedules
		 SET name = $3, description = $4, rules = $5
		 WHERE id = $1 AND workspace_id = $2
		 RETURNING `+scheduleColumns,
		scheduleID, workspaceID, name, description, rulesJSON,
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
