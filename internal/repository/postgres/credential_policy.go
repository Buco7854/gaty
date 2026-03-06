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

type credentialPolicyRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialPolicyRepository(pool *pgxpool.Pool) repository.CredentialPolicyRepository {
	return &credentialPolicyRepository{pool: pool}
}

func (r *credentialPolicyRepository) List(ctx context.Context, credentialID uuid.UUID) ([]model.CredentialPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT subject_id, gate_id, permission_code FROM access_policies
		 WHERE subject_type = 'credential' AND subject_id = $1
		 ORDER BY gate_id, permission_code`,
		credentialID,
	)
	if err != nil {
		return nil, fmt.Errorf("list credential policies: %w", err)
	}
	defer rows.Close()

	var result []model.CredentialPolicy
	for rows.Next() {
		var p model.CredentialPolicy
		if err := rows.Scan(&p.CredentialID, &p.GateID, &p.PermissionCode); err != nil {
			return nil, fmt.Errorf("scan credential policy: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *credentialPolicyRepository) HasAny(ctx context.Context, credentialID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM access_policies WHERE subject_type = 'credential' AND subject_id = $1)`,
		credentialID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check any credential policy: %w", err)
	}
	return exists, nil
}

func (r *credentialPolicyRepository) HasPermission(ctx context.Context, credentialID, gateID uuid.UUID, permCode string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM access_policies
		  WHERE subject_type = 'credential' AND subject_id = $1 AND gate_id = $2 AND permission_code = $3)`,
		credentialID, gateID, permCode,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check credential permission: %w", err)
	}
	return exists, nil
}

func (r *credentialPolicyRepository) Grant(ctx context.Context, credentialID, gateID uuid.UUID, permCode string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO access_policies (subject_type, subject_id, gate_id, permission_code)
		 VALUES ('credential', $1, $2, $3) ON CONFLICT DO NOTHING`,
		credentialID, gateID, permCode,
	)
	if err != nil {
		return fmt.Errorf("grant credential policy: %w", err)
	}
	return nil
}

func (r *credentialPolicyRepository) RevokeAll(ctx context.Context, credentialID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM access_policies WHERE subject_type = 'credential' AND subject_id = $1`,
		credentialID,
	)
	if err != nil {
		return fmt.Errorf("revoke all credential policies: %w", err)
	}
	return nil
}

func (r *credentialPolicyRepository) GetScheduleID(ctx context.Context, credentialID uuid.UUID) (uuid.UUID, error) {
	var scheduleID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT schedule_id FROM schedule_links
		 WHERE subject_type = 'credential' AND subject_id = $1 AND gate_id IS NULL`,
		credentialID,
	).Scan(&scheduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, repository.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get credential schedule: %w", err)
	}
	return scheduleID, nil
}

func (r *credentialPolicyRepository) SetSchedule(ctx context.Context, credentialID, scheduleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO schedule_links (subject_type, subject_id, gate_id, schedule_id)
		 VALUES ('credential', $1, NULL, $2)
		 ON CONFLICT (subject_type, subject_id) WHERE gate_id IS NULL
		 DO UPDATE SET schedule_id = EXCLUDED.schedule_id`,
		credentialID, scheduleID,
	)
	if err != nil {
		return fmt.Errorf("set credential schedule: %w", err)
	}
	return nil
}

func (r *credentialPolicyRepository) RemoveSchedule(ctx context.Context, credentialID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM schedule_links WHERE subject_type = 'credential' AND subject_id = $1 AND gate_id IS NULL`,
		credentialID,
	)
	if err != nil {
		return fmt.Errorf("remove credential schedule: %w", err)
	}
	return nil
}
