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

// applyGateJSON populates the JSON-derived fields of g from raw database bytes.
func applyGateJSON(g *model.Gate, rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules, rawCustomStatuses, rawStatusTransitions []byte) {
	g.OpenConfig = unmarshalActionConfig(rawOpen)
	g.CloseConfig = unmarshalActionConfig(rawClose)
	g.StatusConfig = unmarshalActionConfig(rawStatus)
	if len(rawMeta) > 0 {
		_ = json.Unmarshal(rawMeta, &g.StatusMetadata)
	}
	g.MetaConfig = unmarshalMetaConfig(rawMetaCfg)
	g.StatusRules = unmarshalStatusRules(rawStatusRules)
	if len(rawCustomStatuses) > 0 {
		_ = json.Unmarshal(rawCustomStatuses, &g.CustomStatuses)
	}
	if len(rawStatusTransitions) > 0 {
		_ = json.Unmarshal(rawStatusTransitions, &g.StatusTransitions)
	}
}

// scanGate fills g from a row that yields colsFull columns.
func scanGate(s rowScanner, g *model.Gate) error {
	var rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules, rawCustomStatuses, rawStatusTransitions []byte
	err := s.Scan(
		&g.ID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt,
		&rawMeta, &rawMetaCfg, &rawStatusRules,
		&rawCustomStatuses, &g.TTLSeconds, &rawStatusTransitions,
		&g.CreatedAt,
	)
	if err != nil {
		return err
	}
	applyGateJSON(g, rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules, rawCustomStatuses, rawStatusTransitions)
	return nil
}

const colsFull = `id, name, integration_type, integration_config,
                  open_config, close_config, status_config,
                  status, last_seen_at, status_metadata, meta_config, status_rules,
                  custom_statuses, ttl_seconds, status_transitions, created_at`

func marshalJSONSlice[T any](v []T, fallback string) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(fallback)
	}
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

func (r *gateRepository) Create(ctx context.Context, p repository.CreateGateParams) (*model.Gate, error) {
	if p.IntegrationConfig == nil {
		p.IntegrationConfig = map[string]any{}
	}

	var g model.Gate
	var token string
	var rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules, rawCustomStatuses, rawStatusTransitions []byte
	err := r.pool.QueryRow(ctx,
		`INSERT INTO gates (name, integration_type, integration_config,
		                    open_config, close_config, status_config, meta_config, status_rules, custom_statuses, ttl_seconds, status_transitions)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING gate_token, `+colsFull,
		p.Name, p.IntegrationType, p.IntegrationConfig,
		marshalActionConfig(p.OpenConfig),
		marshalActionConfig(p.CloseConfig),
		marshalActionConfig(p.StatusConfig),
		marshalJSONSlice(p.MetaConfig, "[]"),
		marshalJSONSlice(p.StatusRules, "[]"),
		marshalJSONSlice(p.CustomStatuses, "[]"),
		p.TTLSeconds,
		marshalJSONSlice(p.StatusTransitions, "[]"),
	).Scan(
		&token,
		&g.ID, &g.Name,
		&g.IntegrationType, &g.IntegrationConfig,
		&rawOpen, &rawClose, &rawStatus,
		&g.Status, &g.LastSeenAt,
		&rawMeta, &rawMetaCfg, &rawStatusRules,
		&rawCustomStatuses, &g.TTLSeconds, &rawStatusTransitions,
		&g.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create gate: %w", err)
	}
	applyGateJSON(&g, rawOpen, rawClose, rawStatus, rawMeta, rawMetaCfg, rawStatusRules, rawCustomStatuses, rawStatusTransitions)
	g.GateToken = &token
	return &g, nil
}

func (r *gateRepository) GetByID(ctx context.Context, gateID uuid.UUID) (*model.Gate, error) {
	var g model.Gate
	err := scanGate(
		r.pool.QueryRow(ctx, `SELECT `+colsFull+` FROM gates WHERE id = $1`, gateID),
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

func (r *gateRepository) GetByIDPublic(ctx context.Context, gateID uuid.UUID) (*model.Gate, error) {
	// Same as GetByID in single-instance mode.
	return r.GetByID(ctx, gateID)
}

func (r *gateRepository) GetPublicInfo(ctx context.Context, gateID uuid.UUID) (*repository.GatePublicInfo, error) {
	info := &repository.GatePublicInfo{}
	var rawMetaCfg, rawStatusMeta []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, name,
		        COALESCE(open_config->>'type', 'NONE') <> 'NONE',
		        COALESCE(close_config->>'type', 'NONE') <> 'NONE',
		        status, meta_config, status_metadata
		 FROM gates WHERE id = $1`,
		gateID,
	).Scan(&info.GateID, &info.GateName,
		&info.HasOpenAction, &info.HasCloseAction, &info.Status, &rawMetaCfg, &rawStatusMeta)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get gate public info: %w", err)
	}
	info.MetaConfig = unmarshalMetaConfig(rawMetaCfg)
	if len(rawStatusMeta) > 0 {
		_ = json.Unmarshal(rawStatusMeta, &info.StatusMetadata)
	}
	return info, nil
}

func (r *gateRepository) Update(ctx context.Context, gateID uuid.UUID, p repository.UpdateGateParams) (*model.Gate, error) {
	sets := []string{}
	args := []any{gateID}
	n := 2

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
	if p.CustomStatuses != nil {
		sets = append(sets, fmt.Sprintf("custom_statuses = $%d::jsonb", n))
		args = append(args, marshalJSONSlice(p.CustomStatuses, "[]"))
		n++
	}
	if p.TTLSeconds.Sent {
		if p.TTLSeconds.Null {
			sets = append(sets, fmt.Sprintf("ttl_seconds = $%d", n))
			args = append(args, nil)
		} else {
			sets = append(sets, fmt.Sprintf("ttl_seconds = $%d", n))
			args = append(args, p.TTLSeconds.Value)
		}
		n++
	}
	if p.StatusTransitions != nil {
		sets = append(sets, fmt.Sprintf("status_transitions = $%d::jsonb", n))
		args = append(args, marshalJSONSlice(p.StatusTransitions, "[]"))
		n++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, gateID)
	}

	var g model.Gate
	err := scanGate(
		r.pool.QueryRow(ctx,
			`UPDATE gates SET `+strings.Join(sets, ", ")+` WHERE id = $1 RETURNING `+colsFull,
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

func (r *gateRepository) Delete(ctx context.Context, gateID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM gates WHERE id = $1`, gateID)
	if err != nil {
		return fmt.Errorf("delete gate: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *gateRepository) List(ctx context.Context, role model.Role, memberID uuid.UUID, p model.PaginationParams) ([]model.Gate, int, error) {
	p = p.Normalize()
	isAdmin := role == model.RoleAdmin

	var (
		countQuery string
		dataQuery  string
		countArgs  []any
		dataArgs   []any
	)

	if isAdmin {
		countQuery = `SELECT COUNT(*) FROM gates`
		countArgs = nil
		dataQuery = `SELECT ` + colsFull + ` FROM gates ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		dataArgs = []any{p.Limit, p.Offset}
	} else {
		countQuery = `SELECT COUNT(DISTINCT g.id) FROM gates g
		              JOIN access_policies p ON p.gate_id = g.id AND p.subject_type = 'membership' AND p.subject_id = $1`
		countArgs = []any{memberID}
		dataQuery = `SELECT DISTINCT g.id, g.name, g.integration_type, g.integration_config,
		                g.open_config, g.close_config, g.status_config,
		                g.status, g.last_seen_at, g.status_metadata, g.meta_config, g.status_rules,
		                g.custom_statuses, g.ttl_seconds, g.status_transitions, g.created_at
		         FROM gates g
		         JOIN access_policies p ON p.gate_id = g.id AND p.subject_type = 'membership' AND p.subject_id = $1
		         ORDER BY g.created_at DESC LIMIT $2 OFFSET $3`
		dataArgs = []any{memberID, p.Limit, p.Offset}
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count gates: %w", err)
	}

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list gates: %w", err)
	}
	defer rows.Close()

	var result []model.Gate
	for rows.Next() {
		var g model.Gate
		if err := scanGate(rows, &g); err != nil {
			return nil, 0, fmt.Errorf("scan gate: %w", err)
		}
		result = append(result, g)
	}
	return result, total, rows.Err()
}

func (r *gateRepository) ListIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM gates`)
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

func (r *gateRepository) GetToken(ctx context.Context, gateID uuid.UUID) (string, error) {
	var token string
	err := r.pool.QueryRow(ctx,
		`SELECT gate_token FROM gates WHERE id = $1`, gateID,
	).Scan(&token)
	if err == pgx.ErrNoRows {
		return "", repository.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get gate token: %w", err)
	}
	return token, nil
}

func (r *gateRepository) SetToken(ctx context.Context, gateID uuid.UUID, token string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE gates SET gate_token = $2 WHERE id = $1`,
		gateID, token,
	)
	if err != nil {
		return fmt.Errorf("set gate token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
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
	defaultTTLSeconds := int(ttl.Seconds())

	rows, err := r.pool.Query(ctx,
		`UPDATE gates
		 SET status = $1
		 WHERE last_seen_at IS NOT NULL
		   AND last_seen_at < NOW() - INTERVAL '1 second' * COALESCE(ttl_seconds, $2)
		   AND status NOT IN ($1, $3, $4)
		 RETURNING id`,
		string(model.GateStatusUnresponsive),
		defaultTTLSeconds,
		string(model.GateStatusUnknown),
		string(model.GateStatusOffline),
	)
	if err != nil {
		return nil, fmt.Errorf("mark unresponsive: %w", err)
	}
	defer rows.Close()

	var results []repository.UnresponsiveGate
	for rows.Next() {
		var g repository.UnresponsiveGate
		if err := rows.Scan(&g.GateID); err != nil {
			return nil, err
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

func (r *gateRepository) ListWebhookGates(ctx context.Context) ([]model.Gate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+colsFull+` FROM gates WHERE status_config->>'type' = 'HTTP_WEBHOOK'`,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhook gates: %w", err)
	}
	defer rows.Close()

	var gates []model.Gate
	for rows.Next() {
		var g model.Gate
		if err := scanGate(rows, &g); err != nil {
			return nil, fmt.Errorf("scan webhook gate: %w", err)
		}
		gates = append(gates, g)
	}
	return gates, rows.Err()
}

func (r *gateRepository) ListTransitionCandidates(ctx context.Context) ([]model.Gate, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+colsFull+` FROM gates
		 WHERE status_transitions != '[]'::jsonb
		   AND last_seen_at IS NOT NULL`,
	)
	if err != nil {
		return nil, fmt.Errorf("list transition candidates: %w", err)
	}
	defer rows.Close()

	var gates []model.Gate
	for rows.Next() {
		var g model.Gate
		if err := scanGate(rows, &g); err != nil {
			return nil, fmt.Errorf("scan transition candidate: %w", err)
		}
		gates = append(gates, g)
	}
	return gates, rows.Err()
}
