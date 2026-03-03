package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomDomainRepository struct {
	pool *pgxpool.Pool
}

func NewCustomDomainRepository(pool *pgxpool.Pool) *CustomDomainRepository {
	return &CustomDomainRepository{pool: pool}
}

const domainCols = `id, gate_id, workspace_id, domain, dns_challenge_token, verified_at, created_at`

func scanDomain(row pgx.Row) (*model.CustomDomain, error) {
	d := &model.CustomDomain{}
	err := row.Scan(&d.ID, &d.GateID, &d.WorkspaceID, &d.Domain, &d.DNSChallengeToken, &d.VerifiedAt, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan custom domain: %w", err)
	}
	return d, nil
}

func (r *CustomDomainRepository) Create(ctx context.Context, gateID, workspaceID uuid.UUID, domain string) (*model.CustomDomain, error) {
	d, err := scanDomain(r.pool.QueryRow(ctx,
		`INSERT INTO custom_domains (gate_id, workspace_id, domain)
		 VALUES ($1, $2, $3)
		 RETURNING `+domainCols,
		gateID, workspaceID, domain,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create custom domain: %w", err)
	}
	return d, nil
}

func (r *CustomDomainRepository) GetByID(ctx context.Context, domainID, gateID uuid.UUID) (*model.CustomDomain, error) {
	return scanDomain(r.pool.QueryRow(ctx,
		`SELECT `+domainCols+` FROM custom_domains WHERE id = $1 AND gate_id = $2`,
		domainID, gateID,
	))
}

func (r *CustomDomainRepository) GetByDomain(ctx context.Context, domain string) (*model.CustomDomain, error) {
	return scanDomain(r.pool.QueryRow(ctx,
		`SELECT `+domainCols+` FROM custom_domains WHERE domain = $1`,
		domain,
	))
}

func (r *CustomDomainRepository) ListByGate(ctx context.Context, gateID uuid.UUID) ([]*model.CustomDomain, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+domainCols+` FROM custom_domains WHERE gate_id = $1 ORDER BY created_at DESC`,
		gateID,
	)
	if err != nil {
		return nil, fmt.Errorf("list custom domains: %w", err)
	}
	defer rows.Close()

	var result []*model.CustomDomain
	for rows.Next() {
		d := &model.CustomDomain{}
		if err := rows.Scan(&d.ID, &d.GateID, &d.WorkspaceID, &d.Domain, &d.DNSChallengeToken, &d.VerifiedAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan custom domain row: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// ListAllVerified returns all verified domains. Used by external proxy automation.
func (r *CustomDomainRepository) ListAllVerified(ctx context.Context) ([]*model.CustomDomain, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+domainCols+` FROM custom_domains WHERE verified_at IS NOT NULL ORDER BY domain`,
	)
	if err != nil {
		return nil, fmt.Errorf("list verified domains: %w", err)
	}
	defer rows.Close()

	var result []*model.CustomDomain
	for rows.Next() {
		d := &model.CustomDomain{}
		if err := rows.Scan(&d.ID, &d.GateID, &d.WorkspaceID, &d.Domain, &d.DNSChallengeToken, &d.VerifiedAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan verified domain row: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// SetVerified marks a domain as verified at the given time.
func (r *CustomDomainRepository) SetVerified(ctx context.Context, domainID uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE custom_domains SET verified_at = $2 WHERE id = $1`,
		domainID, at,
	)
	if err != nil {
		return fmt.Errorf("set domain verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CustomDomainRepository) Delete(ctx context.Context, domainID, gateID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM custom_domains WHERE id = $1 AND gate_id = $2`,
		domainID, gateID,
	)
	if err != nil {
		return fmt.Errorf("delete custom domain: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
