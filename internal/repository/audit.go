package repository

import (
	"context"

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

// Insert is a no-op until the audit_logs table is re-added in a future migration.
func (r *AuditRepository) Insert(_ context.Context, _ AuditEntry) error {
	return nil
}
