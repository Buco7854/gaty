package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

type AuditEntry struct {
	WorkspaceID uuid.UUID
	GateID      *uuid.UUID
	UserID      *uuid.UUID
	Action      string
	IP          string
}

func (r *AuditRepository) Insert(ctx context.Context, e AuditEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_logs (workspace_id, gate_id, user_id, action, ip_address)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''))`,
		e.WorkspaceID, e.GateID, e.UserID, e.Action, e.IP,
	)
	if err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}
