package repository

import (
	"context"
	"encoding/json"
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

// marshalActionConfig serialises an ActionConfig to JSON for a JSONB parameter.
// nil input → nil []byte (SQL NULL).
func marshalActionConfig(cfg *model.ActionConfig) json.RawMessage {
	if cfg == nil {
		return nil
	}
	b, _ := json.Marshal(cfg)
	return json.RawMessage(b)
}

// unmarshalActionConfig deserialises raw JSON bytes into an ActionConfig.
// nil/empty input → nil.
func unmarshalActionConfig(data []byte) *model.ActionConfig {
	if len(data) == 0 {
		return nil
	}
	var cfg model.ActionConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}

// scanGateRow populates a Gate from a row that SELECTs the full column set (colsFull).
func scanGateRow(row pgx.Row, g *model.Gate) error {
	var rawOpen, rawClose, rawStatus []byte
	err := row.Scan(
		&g.ID, &g.WorkspaceID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt, &g.CreatedAt,
	)
	if err != nil {
		return err
	}
	g.OpenConfig = unmarshalActionConfig(rawOpen)
	g.CloseConfig = unmarshalActionConfig(rawClose)
	g.StatusConfig = unmarshalActionConfig(rawStatus)
	return nil
}

// scanGateRows populates a Gate from a Rows cursor using the full column set.
func scanGateRows(rows pgx.Rows, g *model.Gate) error {
	var rawOpen, rawClose, rawStatus []byte
	err := rows.Scan(
		&g.ID, &g.WorkspaceID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt, &g.CreatedAt,
	)
	if err != nil {
		return err
	}
	g.OpenConfig = unmarshalActionConfig(rawOpen)
	g.CloseConfig = unmarshalActionConfig(rawClose)
	g.StatusConfig = unmarshalActionConfig(rawStatus)
	return nil
}

const colsFull = `id, workspace_id, name, integration_type, integration_config,
                  open_config, close_config, status_config,
                  status, last_seen_at, created_at`

// CreateGateParams holds all parameters for creating a new gate.
type CreateGateParams struct {
	Name              string
	IntegrationType   model.GateIntegrationType
	IntegrationConfig map[string]any
	OpenConfig        *model.ActionConfig
	CloseConfig       *model.ActionConfig
	StatusConfig      *model.ActionConfig
}

func (r *GateRepository) Create(ctx context.Context, wsID uuid.UUID, p CreateGateParams) (*model.Gate, error) {
	if p.IntegrationConfig == nil {
		p.IntegrationConfig = map[string]any{}
	}
	var g model.Gate
	err := scanGateRow(
		r.pool.QueryRow(ctx,
			`INSERT INTO gates (workspace_id, name, integration_type, integration_config, open_config, close_config, status_config)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING `+colsFull,
			wsID, p.Name, p.IntegrationType, p.IntegrationConfig,
			marshalActionConfig(p.OpenConfig),
			marshalActionConfig(p.CloseConfig),
			marshalActionConfig(p.StatusConfig),
		),
		&g,
	)
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}
	return &g, nil
}

func (r *GateRepository) GetByID(ctx context.Context, gateID, wsID uuid.UUID) (*model.Gate, error) {
	var g model.Gate
	err := scanGateRow(
		r.pool.QueryRow(ctx,
			`SELECT `+colsFull+` FROM gates WHERE id = $1 AND workspace_id = $2`,
			gateID, wsID,
		),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate: %w", err)
	}
	return &g, nil
}

// UpdateGateParams holds the fields that can be updated on a gate.
type UpdateGateParams struct {
	Name         string
	OpenConfig   *model.ActionConfig
	CloseConfig  *model.ActionConfig
	StatusConfig *model.ActionConfig
}

func (r *GateRepository) Update(ctx context.Context, gateID, wsID uuid.UUID, p UpdateGateParams) (*model.Gate, error) {
	var g model.Gate
	err := scanGateRow(
		r.pool.QueryRow(ctx,
			`UPDATE gates
			 SET name = $3, open_config = $4, close_config = $5, status_config = $6
			 WHERE id = $1 AND workspace_id = $2
			 RETURNING `+colsFull,
			gateID, wsID, p.Name,
			marshalActionConfig(p.OpenConfig),
			marshalActionConfig(p.CloseConfig),
			marshalActionConfig(p.StatusConfig),
		),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update gate: %w", err)
	}
	return &g, nil
}

func (r *GateRepository) Delete(ctx context.Context, gateID, wsID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM gates WHERE id = $1 AND workspace_id = $2`,
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

// GetByIDPublic loads a gate by ID alone (no workspace constraint).
// Used by the public PIN unlock endpoint.
func (r *GateRepository) GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error) {
	var g model.Gate
	err := scanGateRow(
		r.pool.QueryRow(ctx,
			`SELECT `+colsFull+` FROM gates WHERE id = $1`,
			gateID,
		),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate public: %w", err)
	}
	return &g, nil
}

func (r *GateRepository) UpdateStatus(ctx context.Context, gateID uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE gates SET status = $2, last_seen_at = NOW() WHERE id = $1`,
		gateID, status,
	)
	if err != nil {
		return fmt.Errorf("update gate status: %w", err)
	}
	return nil
}

// ListForWorkspace returns gates for a workspace.
// OWNER/ADMIN see all gates; MEMBER sees only gates they have at least one policy on.
func (r *GateRepository) ListForWorkspace(ctx context.Context, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID) ([]model.Gate, error) {
	isAdmin := role == model.RoleOwner || role == model.RoleAdmin

	var (
		query string
		args  []any
	)

	if isAdmin {
		query = `SELECT ` + colsFull + ` FROM gates WHERE workspace_id = $1 ORDER BY created_at DESC`
		args = []any{wsID}
	} else {
		query = `SELECT DISTINCT g.id, g.workspace_id, g.name, g.integration_type, g.integration_config,
		                g.open_config, g.close_config, g.status_config,
		                g.status, g.last_seen_at, g.created_at
		         FROM gates g
		         JOIN membership_policies p ON p.gate_id = g.id AND p.membership_id = $2
		         WHERE g.workspace_id = $1
		         ORDER BY g.created_at DESC`
		args = []any{wsID, membershipID}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list gates: %w", err)
	}
	defer rows.Close()

	var result []model.Gate
	for rows.Next() {
		var g model.Gate
		if err := scanGateRows(rows, &g); err != nil {
			return nil, fmt.Errorf("scan gate: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
