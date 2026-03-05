package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) AuditRepository {
	return &pgAuditRepository{pool: pool}
}

type AuditEntry struct {
	WorkspaceID uuid.UUID
	GateID      *uuid.UUID
	UserID      *uuid.UUID
	Action      string
	IP          string
}

// Insert is a no-op until the audit_logs table is re-added in a future migration.
func (r *pgAuditRepository) Insert(_ context.Context, _ AuditEntry) error {
	return nil
}
