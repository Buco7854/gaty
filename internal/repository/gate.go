package repository

import (
	"context"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GateRepository struct {
	pool *pgxpool.Pool
}

func NewGateRepository(pool *pgxpool.Pool) *GateRepository {
	return &GateRepository{pool: pool}
}

func (r *GateRepository) ListForWorkspace(ctx context.Context, wsID, userID uuid.UUID, role model.WorkspaceRole) ([]model.Gate, error) {
	isAdmin := role == model.RoleOwner || role == model.RoleAdmin

	query := `SELECT id, workspace_id, name, integration_type, status, last_seen_at, created_at
	          FROM gates WHERE workspace_id = $1 AND deleted_at IS NULL
	          ORDER BY created_at DESC`
	args := []any{wsID}

	if !isAdmin {
		query = `SELECT DISTINCT g.id, g.workspace_id, g.name, g.integration_type, g.status, g.last_seen_at, g.created_at
		         FROM gates g
		         JOIN gate_user_policies p ON p.gate_id = g.id AND p.user_id = $2
		         WHERE g.workspace_id = $1 AND g.deleted_at IS NULL
		         ORDER BY g.created_at DESC`
		args = append(args, userID)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list gates: %w", err)
	}
	defer rows.Close()

	var result []model.Gate
	for rows.Next() {
		var g model.Gate
		if err := rows.Scan(&g.ID, &g.WorkspaceID, &g.Name, &g.IntegrationType, &g.Status, &g.LastSeenAt, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan gate: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
