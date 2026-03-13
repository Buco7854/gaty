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

type policyRepository struct {
	pool *pgxpool.Pool
}

func NewPolicyRepository(pool *pgxpool.Pool) repository.PolicyRepository {
	return &policyRepository{pool: pool}
}

func (r *policyRepository) Grant(ctx context.Context, memberID, gateID uuid.UUID, permCode string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO access_policies (subject_type, subject_id, gate_id, permission_code)
		 VALUES ('membership', $1, $2, $3) ON CONFLICT DO NOTHING`,
		memberID, gateID, permCode,
	)
	if err != nil {
		return fmt.Errorf("grant policy: %w", err)
	}
	return nil
}

func (r *policyRepository) Revoke(ctx context.Context, memberID, gateID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM access_policies WHERE subject_type = 'membership' AND subject_id = $1 AND gate_id = $2`,
		memberID, gateID,
	)
	if err != nil {
		return fmt.Errorf("revoke policy: %w", err)
	}
	return nil
}

func (r *policyRepository) RevokePermission(ctx context.Context, memberID, gateID uuid.UUID, permCode string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM access_policies
		 WHERE subject_type = 'membership' AND subject_id = $1 AND gate_id = $2 AND permission_code = $3`,
		memberID, gateID, permCode,
	)
	if err != nil {
		return fmt.Errorf("revoke permission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *policyRepository) HasPermission(ctx context.Context, memberID, gateID uuid.UUID, permCode string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM access_policies
		  WHERE subject_type = 'membership' AND subject_id = $1 AND gate_id = $2 AND permission_code = $3)`,
		memberID, gateID, permCode,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return exists, nil
}

func (r *policyRepository) HasAnyPermission(ctx context.Context, memberID, gateID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM access_policies
		  WHERE subject_type = 'membership' AND subject_id = $1 AND gate_id = $2)`,
		memberID, gateID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check any permission: %w", err)
	}
	return exists, nil
}

func (r *policyRepository) HasPermissionOnAnyGate(ctx context.Context, memberID uuid.UUID, permCode string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM access_policies
			WHERE subject_type = 'membership' AND subject_id = $1 AND permission_code = $2
		)`,
		memberID, permCode,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check permission on any gate: %w", err)
	}
	return exists, nil
}

func (r *policyRepository) ListForGate(ctx context.Context, gateID uuid.UUID, p model.PaginationParams) ([]model.MemberPolicy, int, error) {
	p = p.Normalize()

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM access_policies WHERE subject_type = 'membership' AND gate_id = $1`,
		gateID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count policies: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT subject_id, gate_id, permission_code FROM access_policies
		 WHERE subject_type = 'membership' AND gate_id = $1
		 ORDER BY subject_id, permission_code
		 LIMIT $2 OFFSET $3`,
		gateID, p.Limit, p.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	var result []model.MemberPolicy
	for rows.Next() {
		var mp model.MemberPolicy
		if err := rows.Scan(&mp.MemberID, &mp.GateID, &mp.PermissionCode); err != nil {
			return nil, 0, fmt.Errorf("scan policy: %w", err)
		}
		result = append(result, mp)
	}
	return result, total, rows.Err()
}

func (r *policyRepository) ListForMember(ctx context.Context, memberID uuid.UUID, p model.PaginationParams) ([]model.MemberPolicy, int, error) {
	p = p.Normalize()

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM access_policies WHERE subject_type = 'membership' AND subject_id = $1`,
		memberID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count member policies: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT subject_id, gate_id, permission_code FROM access_policies
		 WHERE subject_type = 'membership' AND subject_id = $1
		 ORDER BY gate_id, permission_code
		 LIMIT $2 OFFSET $3`,
		memberID, p.Limit, p.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list member policies: %w", err)
	}
	defer rows.Close()

	var result []model.MemberPolicy
	for rows.Next() {
		var mp model.MemberPolicy
		if err := rows.Scan(&mp.MemberID, &mp.GateID, &mp.PermissionCode); err != nil {
			return nil, 0, fmt.Errorf("scan policy: %w", err)
		}
		result = append(result, mp)
	}
	return result, total, rows.Err()
}

func (r *policyRepository) GetScheduleID(ctx context.Context, memberID, gateID uuid.UUID) (uuid.UUID, error) {
	var scheduleID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT schedule_id FROM schedule_links
		 WHERE subject_type = 'membership' AND subject_id = $1 AND gate_id = $2`,
		memberID, gateID,
	).Scan(&scheduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, repository.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get member gate schedule: %w", err)
	}
	return scheduleID, nil
}

func (r *policyRepository) SetSchedule(ctx context.Context, memberID, gateID, scheduleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO schedule_links (subject_type, subject_id, gate_id, schedule_id)
		 VALUES ('membership', $1, $2, $3)
		 ON CONFLICT (subject_type, subject_id, gate_id) WHERE gate_id IS NOT NULL
		 DO UPDATE SET schedule_id = EXCLUDED.schedule_id`,
		memberID, gateID, scheduleID,
	)
	if err != nil {
		return fmt.Errorf("set member gate schedule: %w", err)
	}
	return nil
}

func (r *policyRepository) RemoveSchedule(ctx context.Context, memberID, gateID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM schedule_links WHERE subject_type = 'membership' AND subject_id = $1 AND gate_id = $2`,
		memberID, gateID,
	)
	if err != nil {
		return fmt.Errorf("remove member gate schedule: %w", err)
	}
	return nil
}
