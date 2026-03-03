package repository

import (
	"context"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GateRepository struct {
	pool *pgxpool.Pool
}

func NewGateRepository(pool *pgxpool.Pool) *GateRepository {
	return &GateRepository{pool: pool}
}

func (r *GateRepository) Create(ctx context.Context, wsID uuid.UUID, name string, intType model.GateIntegrationType, config map[string]any) (*model.Gate, error) {
	if config == nil {
		config = map[string]any{}
	}
	var g model.Gate
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gates (workspace_id, name, integration_type, integration_config)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, workspace_id, name, integration_type, integration_config, status, last_seen_at, created_at`,
		wsID, name, intType, config,
	).Scan(&g.ID, &g.WorkspaceID, &g.Name, &g.IntegrationType, &g.IntegrationConfig, &g.Status, &g.LastSeenAt, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}
	return &g, nil
}

func (r *GateRepository) GetByID(ctx context.Context, gateID, wsID uuid.UUID) (*model.Gate, error) {
	var g model.Gate
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, integration_type, integration_config, status, last_seen_at, created_at
		 FROM gates WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		gateID, wsID,
	).Scan(&g.ID, &g.WorkspaceID, &g.Name, &g.IntegrationType, &g.IntegrationConfig, &g.Status, &g.LastSeenAt, &g.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate: %w", err)
	}
	return &g, nil
}

func (r *GateRepository) Update(ctx context.Context, gateID, wsID uuid.UUID, name string, config map[string]any) (*model.Gate, error) {
	var g model.Gate
	err := r.pool.QueryRow(ctx,
		`UPDATE gates SET name = $3, integration_config = $4
		 WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
		 RETURNING id, workspace_id, name, integration_type, integration_config, status, last_seen_at, created_at`,
		gateID, wsID, name, config,
	).Scan(&g.ID, &g.WorkspaceID, &g.Name, &g.IntegrationType, &g.IntegrationConfig, &g.Status, &g.LastSeenAt, &g.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update gate: %w", err)
	}
	return &g, nil
}

func (r *GateRepository) SoftDelete(ctx context.Context, gateID, wsID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gates SET deleted_at = NOW() WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
		gateID, wsID,
	)
	if err != nil {
		return fmt.Errorf("delete gate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GateRepository) UpdateStatus(ctx context.Context, gateID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE gates SET status = $2, last_seen_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		gateID, status,
	)
	if err != nil {
		return fmt.Errorf("update gate status: %w", err)
	}
	return nil
}

func (r *GateRepository) ListForWorkspace(ctx context.Context, wsID, userID uuid.UUID, role model.WorkspaceRole) ([]model.Gate, error) {
	isAdmin := role == model.RoleOwner || role == model.RoleAdmin

	query := `SELECT id, workspace_id, name, integration_type, integration_config, status, last_seen_at, created_at
	          FROM gates WHERE workspace_id = $1 AND deleted_at IS NULL
	          ORDER BY created_at DESC`
	args := []any{wsID}

	if !isAdmin {
		query = `SELECT DISTINCT g.id, g.workspace_id, g.name, g.integration_type, g.integration_config, g.status, g.last_seen_at, g.created_at
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
		if err := rows.Scan(&g.ID, &g.WorkspaceID, &g.Name, &g.IntegrationType, &g.IntegrationConfig, &g.Status, &g.LastSeenAt, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan gate: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
