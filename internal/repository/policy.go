package repository

import (
	"context"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PolicyRepository struct {
	pool *pgxpool.Pool
}

func NewPolicyRepository(pool *pgxpool.Pool) *PolicyRepository {
	return &PolicyRepository{pool: pool}
}

func (r *PolicyRepository) List(ctx context.Context, gateID uuid.UUID) ([]model.GatePolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT gate_id, user_id, permission_code FROM gate_user_policies
		 WHERE gate_id = $1 ORDER BY user_id, permission_code`,
		gateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	var result []model.GatePolicy
	for rows.Next() {
		var p model.GatePolicy
		if err := rows.Scan(&p.GateID, &p.UserID, &p.PermissionCode); err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// Grant adds a permission for a user on a gate. Idempotent (ON CONFLICT DO NOTHING).
func (r *PolicyRepository) Grant(ctx context.Context, gateID, userID uuid.UUID, permCode string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO gate_user_policies (gate_id, user_id, permission_code)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		gateID, userID, permCode,
	)
	if err != nil {
		return fmt.Errorf("grant policy: %w", err)
	}
	return nil
}

// Revoke removes all permissions for a user on a gate.
func (r *PolicyRepository) Revoke(ctx context.Context, gateID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM gate_user_policies WHERE gate_id = $1 AND user_id = $2`,
		gateID, userID,
	)
	if err != nil {
		return fmt.Errorf("revoke policy: %w", err)
	}
	return nil
}
