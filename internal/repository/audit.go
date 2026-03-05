package repository

import (
	"context"

	"github.com/google/uuid"
)

// AuditEntry holds the data for a single audit log entry.
type AuditEntry struct {
	WorkspaceID uuid.UUID
	GateID      *uuid.UUID
	UserID      *uuid.UUID
	Action      string
	IP          string
}

// AuditRepository is the data-access contract for audit log entries.
type AuditRepository interface {
	Insert(ctx context.Context, entry AuditEntry) error
}
