package postgres

import (
	"context"

	"github.com/Buco7854/gatie/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type auditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) repository.AuditRepository {
	return &auditRepository{pool: pool}
}

// Insert is a no-op until the audit_logs table is re-added in a future migration.
// TODO: implement once migration for audit_logs table is added.
func (r *auditRepository) Insert(_ context.Context, _ repository.AuditEntry) error {
	return nil
}
