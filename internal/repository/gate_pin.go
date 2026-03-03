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

const gatePinColumns = `id, gate_id, hashed_pin, label, metadata, created_at`

func scanGatePin(row pgx.Row) (*model.GatePin, error) {
	p := &model.GatePin{}
	err := row.Scan(&p.ID, &p.GateID, &p.HashedPin, &p.Label, &p.Metadata, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan gate pin: %w", err)
	}
	return p, nil
}

func (r *GatePinRepository) Create(ctx context.Context, gateID uuid.UUID, hashedPin string, label *string, metadata map[string]any) (*model.GatePin, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	p, err := scanGatePin(r.pool.QueryRow(ctx,
		`INSERT INTO gate_pins (gate_id, hashed_pin, label, metadata)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+gatePinColumns,
		gateID, hashedPin, label, metadata,
	))
	if err != nil {
		return nil, fmt.Errorf("create gate pin: %w", err)
	}
	return p, nil
}

func (r *GatePinRepository) GetByID(ctx context.Context, pinID, gateID uuid.UUID) (*model.GatePin, error) {
	return scanGatePin(r.pool.QueryRow(ctx,
		`SELECT `+gatePinColumns+` FROM gate_pins WHERE id = $1 AND gate_id = $2`,
		pinID, gateID,
	))
}

// List returns all pins for a gate. Used for PIN authentication (bcrypt compare loop).
func (r *GatePinRepository) List(ctx context.Context, gateID uuid.UUID) ([]*model.GatePin, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+gatePinColumns+` FROM gate_pins WHERE gate_id = $1 ORDER BY created_at DESC`,
		gateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list gate pins: %w", err)
	}
	defer rows.Close()

	var result []*model.GatePin
	for rows.Next() {
		p := &model.GatePin{}
		if err := rows.Scan(&p.ID, &p.GateID, &p.HashedPin, &p.Label, &p.Metadata, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan gate pin row: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (r *GatePinRepository) Delete(ctx context.Context, pinID, gateID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM gate_pins WHERE id = $1 AND gate_id = $2`,
		pinID, gateID,
	)
	if err != nil {
		return fmt.Errorf("delete gate pin: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
