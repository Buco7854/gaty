package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GatePinRepository struct {
	pool *pgxpool.Pool
}

func NewGatePinRepository(pool *pgxpool.Pool) *GatePinRepository {
	return &GatePinRepository{pool: pool}
}

const gatePinColumns = `id, gate_id, hashed_pin, label, metadata, schedule_id, created_at`

func scanGatePin(row pgx.Row) (*model.GatePin, error) {
	p := &model.GatePin{}
	err := row.Scan(&p.ID, &p.GateID, &p.HashedPin, &p.Label, &p.Metadata, &p.ScheduleID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan gate pin: %w", err)
	}
	return p, nil
}

func (r *GatePinRepository) Create(ctx context.Context, gateID uuid.UUID, hashedPin string, label string, metadata map[string]any, scheduleID *uuid.UUID) (*model.GatePin, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`INSERT INTO gate_access_codes (gate_id, hashed_pin, label, metadata, schedule_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+gatePinColumns,
		gateID, hashedPin, label, metadata, scheduleID,
	))
	if err != nil {
		return nil, fmt.Errorf("create gate pin: %w", err)
	}
	return p, nil
}

func (r *GatePinRepository) GetByID(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error) {
	return scanGatePin(r.pool.QueryRow(ctx,
		`SELECT `+gatePinColumns+` FROM gate_access_codes WHERE id = $1 AND gate_id = $2`,
		pinID, gateID,
	))
}

// List returns all access codes for a gate. Used for authentication (bcrypt compare loop).
func (r *GatePinRepository) List(ctx context.Context, gateID uuid.UUID) ([]*model.GatePin, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+gatePinColumns+` FROM gate_access_codes WHERE gate_id = $1 ORDER BY created_at DESC`,
		gateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list gate access codes: %w", err)
	}
	defer rows.Close()

	var result []*model.GatePin
	for rows.Next() {
		p := &model.GatePin{}
		if err := rows.Scan(&p.ID, &p.GateID, &p.HashedPin, &p.Label, &p.Metadata, &p.ScheduleID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan gate access code row: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// Update updates the label and metadata of an access code. The hashed value and schedule are never modified.
// Metadata is merged using JSONB || (right side wins for duplicate keys).
// To change the schedule use SetPinSchedule / ClearPinSchedule.
func (r *GatePinRepository) Update(ctx context.Context, pinID, gateID uuid.UUID, label string, metadata map[string]any) (*model.GatePin, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`UPDATE gate_access_codes
		 SET label = $1, metadata = metadata || $2
		 WHERE id = $3 AND gate_id = $4
		 RETURNING `+gatePinColumns,
		label, metadata, pinID, gateID,
	))
	if err != nil {
		return nil, fmt.Errorf("update gate access code: %w", err)
	}
	return p, nil
}

// SetPinSchedule attaches (or replaces) the time-restriction schedule on a PIN.
func (r *GatePinRepository) SetPinSchedule(ctx context.Context, pinID, gateID, scheduleID uuid.UUID) (*model.GatePin, error) {
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`UPDATE gate_access_codes SET schedule_id = $3 WHERE id = $1 AND gate_id = $2 RETURNING `+gatePinColumns,
		pinID, gateID, scheduleID,
	))
	if err != nil {
		return nil, fmt.Errorf("set pin schedule: %w", err)
	}
	return p, nil
}

// ClearPinSchedule removes any time-restriction schedule from a PIN.
func (r *GatePinRepository) ClearPinSchedule(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error) {
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`UPDATE gate_access_codes SET schedule_id = NULL WHERE id = $1 AND gate_id = $2 RETURNING `+gatePinColumns,
		pinID, gateID,
	))
	if err != nil {
		return nil, fmt.Errorf("clear pin schedule: %w", err)
	}
	return p, nil
}

func (r *GatePinRepository) Delete(ctx context.Context, pinID, gateID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM gate_access_codes WHERE id = $1 AND gate_id = $2`,
		pinID, gateID,
	)
	if err != nil {
		return fmt.Errorf("delete gate access code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
