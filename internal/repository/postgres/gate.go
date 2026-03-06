package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type gateRepository struct {
	pool *pgxpool.Pool
}

func NewGateRepository(pool *pgxpool.Pool) repository.GateRepository {
	return &gateRepository{pool: pool}
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

func unmarshalStatusRules(data []byte) []model.StatusRule {
	if len(data) == 0 {
		return nil
	}
	var rules []model.StatusRule
	if json.Unmarshal(data, &rules) != nil {
		return nil
	}
	return rules
}

// rowScanner is satisfied by both pgx.Row (single-row query) and pgx.Rows (multi-row iteration).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanGate fills g from a row that yields colsFull columns.
// Callers handle ErrNoRows themselves (pgx.Row.Scan returns it directly).
func scanGate(s rowScanner, g *model.Gate) error {
	var rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules []byte
	err := s.Scan(
		&g.ID, &g.WorkspaceID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt,
		&rawMeta, &rawMetaCfg, &rawStatusRules,
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
	g.StatusRules = unmarshalStatusRules(rawStatusRules)
	return nil
}

const colsFull = `id, workspace_id, name, integration_type, integration_config,
                  open_config, close_config, status_config,
                  status, last_seen_at, status_metadata, meta_config, status_rules, created_at`

func marshalJSONSlice[T any](v []T, fallback string) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(fallback)
	}
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}


func (r *gateRepository) Create(ctx context.Context, wsID uuid.UUID, p repository.CreateGateParams) (*model.Gate, error) {
	if p.IntegrationConfig == nil {
		p.IntegrationConfig = map[string]any{}
	}

	// Single round-trip: INSERT + RETURNING gate_token + all gate columns.
	var g model.Gate
	var token string
	var rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules []byte
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gates (workspace_id, name, integration_type, integration_config,
		                    open_config, close_config, status_config, meta_config, status_rules)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING gate_token, `+colsFull,
		wsID, p.Name, p.IntegrationType, p.IntegrationConfig,
		marshalActionConfig(p.OpenConfig),
		marshalActionConfig(p.CloseConfig),
		marshalActionConfig(p.StatusConfig),
		marshalJSONSlice(p.MetaConfig, "[]"),
		marshalJSONSlice(p.StatusRules, "[]"),
	).Scan(
		&token,
		&g.ID, &g.WorkspaceID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt,
		&rawMeta, &rawMetaCfg, &rawStatusRules,
		&g.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}
	g.OpenConfig = unmarshalActionConfig(rawOpen)
	g.CloseConfig = unmarshalActionConfig(rawClose)
	g.StatusConfig = unmarshalActionConfig(rawStatus)
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &g.StatusMetadata)
	}
	g.MetaConfig = unmarshalMetaConfig(rawMetaCfg)
	g.StatusRules = unmarshalStatusRules(rawStatusRules)
	g.GateToken = &token
	return &g, nil
}

func (r *gateRepository) GetByID(ctx context.Context, gateID, wsID uuid.UUID) (*model.Gate, error) {
	var g model.Gate
	err := scanGate(
		r.pool.QueryRow(ctx, `SELECT `+colsFull+` FROM gates WHERE id = $1 AND workspace_id = $2`, gateID, wsID),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate: %w", err)
	}
	return &g, nil
}

func (r *gateRepository) Update(ctx context.Context, gateID, wsID uuid.UUID, p repository.UpdateGateParams) (*model.Gate, error) {
	sets := []string{}
	args := []any{gateID, wsID}
	n := 3

	if p.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", n))
		args = append(args, *p.Name)
		n++
	}
	if p.OpenConfig.Sent {
		sets = append(sets, fmt.Sprintf("open_config = $%d::jsonb", n))
		var cfg *model.ActionConfig
		if !p.OpenConfig.Null {
			cfg = &p.OpenConfig.Value
		}
		args = append(args, marshalActionConfig(cfg))
		n++
	}
	if p.CloseConfig.Sent {
		sets = append(sets, fmt.Sprintf("close_config = $%d::jsonb", n))
		var cfg *model.ActionConfig
		if !p.CloseConfig.Null {
			cfg = &p.CloseConfig.Value
		}
		args = append(args, marshalActionConfig(cfg))
		n++
	}
	if p.StatusConfig.Sent {
		sets = append(sets, fmt.Sprintf("status_config = $%d::jsonb", n))
		var cfg *model.ActionConfig
		if !p.StatusConfig.Null {
			cfg = &p.StatusConfig.Value
		}
		args = append(args, marshalActionConfig(cfg))
		n++
	}
	if p.MetaConfig != nil {
		sets = append(sets, fmt.Sprintf("meta_config = $%d::jsonb", n))
		args = append(args, marshalJSONSlice(p.MetaConfig, "[]"))
		n++
	}
	if p.StatusRules != nil {
		sets = append(sets, fmt.Sprintf("status_rules = $%d::jsonb", n))
		args = append(args, marshalJSONSlice(p.StatusRules, "[]"))
		n++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, gateID, wsID)
	}

	var g model.Gate
	err := scanGate(
		r.pool.QueryRow(ctx,
			`UPDATE gates SET `+strings.Join(sets, ", ")+` WHERE id = $1 AND workspace_id = $2 RETURNING `+colsFull,
			args...,
		),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update gate: %w", err)
	}
	return &g, nil
}

func (r *gateRepository) Delete(ctx context.Context, gateID, wsID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM gates WHERE id = $1 AND workspace_id = $2`, gateID, wsID)
	if err != nil {
		return fmt.Errorf("delete gate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *gateRepository) GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error) {
	var g model.Gate
	err := scanGate(
		r.pool.QueryRow(ctx, `SELECT `+colsFull+` FROM gates WHERE id = $1`, gateID),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate public: %w", err)
	}
	return &g, nil
}

func (r *gateRepository) GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*repository.GatePublicInfo, error) {
	info := &repository.GatePublicInfo{}
	err := r.pool.QueryRow(ctx,
		`SELECT g.id, g.name, w.id, w.name,
		        COALESCE(g.open_config->>'type', 'NONE') <> 'NONE',
		        COALESCE(g.close_config->>'type', 'NONE') <> 'NONE'
		 FROM gates g
		 JOIN workspaces w ON w.id = g.workspace_id
		 WHERE g.id = $1`,
		gateID,
	).Scan(&info.GateID, &info.GateName, &info.WorkspaceID, &info.WorkspaceName, &info.HasOpenAction, &info.HasCloseAction)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate public info: %w", err)
	}
	return info, nil
}

func (r *gateRepository) GetByToken(ctx context.Context, gateID uuid.UUID, token string) (*model.Gate, error) {
	var g model.Gate
	err := scanGate(
		r.pool.QueryRow(ctx, `SELECT `+colsFull+` FROM gates WHERE id = $1 AND gate_token = $2`, gateID, token),
		&g,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrUnauthorized
	}
	if err != nil {
		return nil, fmt.Errorf("get gate by token: %w", err)
	}
	return &g, nil
}

func (r *gateRepository) UpdateStatus(ctx context.Context, gateID uuid.UUID, status string, meta map[string]any) error {
	metaJSON := json.RawMessage("{}")
	if len(meta) > 0 {
		b, _ := json.Marshal(meta)
		metaJSON = json.RawMessage(b)
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE gates SET status = $2, last_seen_at = NOW(), status_metadata = $3 WHERE id = $1`,
		gateID, status, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("update gate status: %w", err)
	}
	return nil
}

func (r *gateRepository) MarkUnresponsiveWithIDs(ctx context.Context, ttl time.Duration) ([]repository.UnresponsiveGate, error) {
	// Compute the cutoff timestamp on the application side so it uses the app's clock
	// (consistent with time.Since checks in the model) and avoids fragile string interval formatting.
	cutoff := time.Now().Add(-ttl)

	rows, err := r.pool.Query(ctx,
		`UPDATE gates
		 SET status = $1
		 WHERE last_seen_at IS NOT NULL
		   AND last_seen_at < $2
		   AND status NOT IN ($1, $3, $4)
		 RETURNING id, workspace_id`,
		string(model.GateStatusUnresponsive),
		cutoff,
		string(model.GateStatusUnknown),
		// Don't override "offline": a gate may self-report offline before going quiet,
		// and marking it "unresponsive" would lose that intentional state.
		string(model.GateStatusOffline),
	)
	if err != nil {
		return nil, fmt.Errorf("mark unresponsive: %w", err)
	}
	defer rows.Close()

	var results []repository.UnresponsiveGate
	for rows.Next() {
		var g repository.UnresponsiveGate
		if err := rows.Scan(&g.GateID, &g.WorkspaceID); err != nil {
			return nil, err
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

func (r *gateRepository) GetToken(ctx context.Context, gateID, wsID uuid.UUID) (string, error) {
	var token string
	err := r.pool.QueryRow(ctx,
		`SELECT gate_token FROM gates WHERE id = $1 AND workspace_id = $2`, gateID, wsID,
	).Scan(&token)
	if err == pgx.ErrNoRows {
		return "", repository.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get gate token: %w", err)
	}
	return token, nil
}

func (r *gateRepository) SetToken(ctx context.Context, gateID, wsID uuid.UUID, token string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gates SET gate_token = $3 WHERE id = $1 AND workspace_id = $2`,
		gateID, wsID, token,
	)
	if err != nil {
		return fmt.Errorf("set gate token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *gateRepository) ListIDsForWorkspace(ctx context.Context, wsID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM gates WHERE workspace_id = $1`, wsID)
	if err != nil {
		return nil, fmt.Errorf("list gate ids: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan gate id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *gateRepository) ListForWorkspace(ctx context.Context, wsID uuid.UUID, role model.WorkspaceRole, membershipID uuid.UUID) ([]model.Gate, error) {
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
		                g.status, g.last_seen_at, g.status_metadata, g.meta_config, g.status_rules, g.created_at
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
		if err := scanGate(rows, &g); err != nil {
			return nil, fmt.Errorf("scan gate: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
