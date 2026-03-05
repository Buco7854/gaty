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

func marshalActionConfig(cfg *model.ActionConfig) json.RawMessage {
	if cfg == nil {
		return nil
	}
	b, _ := json.Marshal(cfg)
	return json.RawMessage(b)
}

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

func unmarshalMetaConfig(data []byte) []model.MetaField {
	if len(data) == 0 {
		return nil
	}
	var fields []model.MetaField
	if json.Unmarshal(data, &fields) != nil {
		return nil
	}
	return fields
}

func scanGateRow(row pgx.Row, g *model.Gate) error {
	var rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg []byte
	err := row.Scan(
		&g.ID, &g.WorkspaceID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt,
		&rawMeta, &rawMetaCfg,
		&g.CreatedAt,
	)
	if err != nil {
		return err
	}
	g.OpenConfig = unmarshalActionConfig(rawOpen)
	g.CloseConfig = unmarshalActionConfig(rawClose)
	g.StatusConfig = unmarshalActionConfig(rawStatus)
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &g.StatusMetadata)
	}
	g.MetaConfig = unmarshalMetaConfig(rawMetaCfg)
	return nil
}

func scanGateRows(rows pgx.Rows, g *model.Gate) error {
	var rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg []byte
	err := rows.Scan(
		&g.ID, &g.WorkspaceID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt,
		&rawMeta, &rawMetaCfg,
		&g.CreatedAt,
	)
	if err != nil {
		return err
	}
	g.OpenConfig = unmarshalActionConfig(rawOpen)
	g.CloseConfig = unmarshalActionConfig(rawClose)
	g.StatusConfig = unmarshalActionConfig(rawStatus)
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &g.StatusMetadata)
	}
	g.MetaConfig = unmarshalMetaConfig(rawMetaCfg)
	return nil
}

const colsFull = `id, workspace_id, name, integration_type, integration_config,
                  open_config, close_config, status_config,
                  status, last_seen_at, status_metadata, meta_config, created_at`

// CreateGateParams holds all parameters for creating a new gate.
type CreateGateParams struct {
	Name              string
	IntegrationType   model.GateIntegrationType
	IntegrationConfig map[string]any
	OpenConfig        *model.ActionConfig
	CloseConfig       *model.ActionConfig
	StatusConfig      *model.ActionConfig
	MetaConfig        []model.MetaField
}

func (r *GateRepository) Create(ctx context.Context, wsID uuid.UUID, p CreateGateParams) (*model.Gate, error) {
	if p.IntegrationConfig == nil {
		p.IntegrationConfig = map[string]any{}
	}
	metaCfgJSON := json.RawMessage("[]")
	if len(p.MetaConfig) > 0 {
		b, _ := json.Marshal(p.MetaConfig)
		metaCfgJSON = json.RawMessage(b)
	}

	var gateID uuid.UUID
	var token string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gates (workspace_id, name, integration_type, integration_config,
		                    open_config, close_config, status_config, meta_config)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, gate_token`,
		wsID, p.Name, p.IntegrationType, p.IntegrationConfig,
		marshalActionConfig(p.OpenConfig),
		marshalActionConfig(p.CloseConfig),
		marshalActionConfig(p.StatusConfig),
		metaCfgJSON,
	).Scan(&gateID, &token)
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}

	gate, err := r.GetByID(ctx, gateID, wsID)
	if err != nil {
		return nil, err
	}
	gate.GateToken = &token
	return gate, nil
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
	MetaConfig   []model.MetaField
}

func (r *GateRepository) Update(ctx context.Context, gateID, wsID uuid.UUID, p UpdateGateParams) (*model.Gate, error) {
	metaCfgJSON := json.RawMessage("[]")
	if len(p.MetaConfig) > 0 {
		b, _ := json.Marshal(p.MetaConfig)
		metaCfgJSON = json.RawMessage(b)
	}
	var g model.Gate
	err := scanGateRow(
		r.pool.QueryRow(ctx,
			`UPDATE gates
			 SET name = $3, open_config = $4, close_config = $5, status_config = $6, meta_config = $7
			 WHERE id = $1 AND workspace_id = $2
			 RETURNING `+colsFull,
			gateID, wsID, p.Name,
			marshalActionConfig(p.OpenConfig),
			marshalActionConfig(p.CloseConfig),
			marshalActionConfig(p.StatusConfig),
			metaCfgJSON,
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

// GatePublicInfo holds the gate + workspace context needed for the public PIN pad.
type GatePublicInfo struct {
	GateID        uuid.UUID
	GateName      string
	WorkspaceID   uuid.UUID
	WorkspaceName string
}

// GetPublicInfo returns gate + workspace info for the public PIN pad by gate ID alone.
func (r *GateRepository) GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*GatePublicInfo, error) {
	info := &GatePublicInfo{}
	err := r.pool.QueryRow(ctx,
		`SELECT g.id, g.name, w.id, w.name
		 FROM gates g
		 JOIN workspaces w ON w.id = g.workspace_id
		 WHERE g.id = $1`,
		gateID,
	).Scan(&info.GateID, &info.GateName, &info.WorkspaceID, &info.WorkspaceName)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate public info: %w", err)
	}
	return info, nil
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

// UpdateStatusWithMeta updates the gate status + metadata after validating the gate token.
// Returns ErrUnauthorized if id+token don't match any gate.
func (r *GateRepository) UpdateStatusWithMeta(ctx context.Context, gateID uuid.UUID, token, status string, meta map[string]any) error {
	metaJSON := json.RawMessage("{}")
	if len(meta) > 0 {
		b, _ := json.Marshal(meta)
		metaJSON = json.RawMessage(b)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE gates
		 SET status = $3, last_seen_at = NOW(), status_metadata = $4
		 WHERE id = $1 AND gate_token = $2`,
		gateID, token, status, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("update gate status+meta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUnauthorized
	}
	return nil
}

// GetToken returns the current gate authentication token (admin only).
func (r *GateRepository) GetToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error) {
	var token string
	err := r.pool.QueryRow(ctx,
		`SELECT gate_token FROM gates WHERE id = $1 AND workspace_id = $2`,
		gateID, wsID,
	).Scan(&token)
	if err == pgx.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get gate token: %w", err)
	}
	return token, nil
}

// RotateToken generates a new random token for the gate and returns it.
func (r *GateRepository) RotateToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error) {
	var token string
	err := r.pool.QueryRow(ctx,
		`UPDATE gates
		 SET gate_token = encode(gen_random_bytes(32), 'hex')
		 WHERE id = $1 AND workspace_id = $2
		 RETURNING gate_token`,
		gateID, wsID,
	).Scan(&token)
	if err == pgx.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("rotate gate token: %w", err)
	}
	return token, nil
}

// ListForWorkspace returns gates for a workspace.
// OWNER/ADMIN see all gates; MEMBER sees only gates with at least one policy.
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
		                g.status, g.last_seen_at, g.status_metadata, g.meta_config, g.created_at
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
