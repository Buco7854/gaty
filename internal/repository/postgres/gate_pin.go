package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type gatePinRepository struct {
	pool *pgxpool.Pool
}

func NewGatePinRepository(pool *pgxpool.Pool) repository.GatePinRepository {
	return &gatePinRepository{pool: pool}
}

const gatePinColumns = `id, gate_id, hashed_pin, label, metadata, schedule_id, created_at`

func scanGatePin(row pgx.Row) (*model.GatePin, error) {
	p := &model.GatePin{}
	err := row.Scan(&p.ID, &p.GateID, &p.HashedPin, &p.Label, &p.Metadata, &p.ScheduleID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan gate pin: %w", err)
	}
	return p, nil
}

func (r *gatePinRepository) Create(ctx context.Context, gateID uuid.UUID, hashedPin string, label string, metadata map[string]any, scheduleID *uuid.UUID) (*model.GatePin, error) {
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

func (r *gatePinRepository) GetByID(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error) {
	return scanGatePin(r.pool.QueryRow(ctx,
		`SELECT `+gatePinColumns+` FROM gate_access_codes WHERE id = $1 AND gate_id = $2`,
		pinID, gateID,
	))
}

func (r *gatePinRepository) List(ctx context.Context, gateID uuid.UUID) ([]*model.GatePin, error) {
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

func (r *gatePinRepository) Update(ctx context.Context, pinID, gateID uuid.UUID, label *string, metadata map[string]any) (*model.GatePin, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`UPDATE gate_access_codes
		 SET label = COALESCE($1, label),
		     metadata = metadata || $2
		 WHERE id = $3 AND gate_id = $4
		 RETURNING `+gatePinColumns,
		label, metadata, pinID, gateID,
	))
	if err != nil {
		return nil, fmt.Errorf("update gate access code: %w", err)
	}
	return p, nil
}

func (r *gatePinRepository) SetPinSchedule(ctx context.Context, pinID, gateID, scheduleID uuid.UUID) (*model.GatePin, error) {
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`UPDATE gate_access_codes SET schedule_id = $3 WHERE id = $1 AND gate_id = $2 RETURNING `+gatePinColumns,
		pinID, gateID, scheduleID,
	))
	if err != nil {
		return nil, fmt.Errorf("set pin schedule: %w", err)
	}
	return p, nil
}

func (r *gatePinRepository) ClearPinSchedule(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error) {
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`UPDATE gate_access_codes SET schedule_id = NULL WHERE id = $1 AND gate_id = $2 RETURNING `+gatePinColumns,
		pinID, gateID,
	))
	if err != nil {
		return nil, fmt.Errorf("clear pin schedule: %w", err)
	}
	return p, nil
}

func (r *gatePinRepository) Delete(ctx context.Context, pinID, gateID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM gate_access_codes WHERE id = $1 AND gate_id = $2`,
		pinID, gateID,
	)
	if err != nil {
		return fmt.Errorf("delete gate access code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
