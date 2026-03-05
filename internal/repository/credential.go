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

type pgCredentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) CredentialRepository {
	return &pgCredentialRepository{pool: pool}
}

const credColumns = `id, user_id, type, hashed_value, label, expires_at, metadata, created_at`

func scanCredential(row pgx.Row) (*model.Credential, error) {
	c := &model.Credential{}
	err := row.Scan(&c.ID, &c.UserID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan credential: %w", err)
	}
	return c, nil
}

func (r *pgCredentialRepository) Create(ctx context.Context, userID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.Credential, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	c, err := scanCredential(r.pool.QueryRow(ctx,
		`INSERT INTO credentials (user_id, type, hashed_value, label, expires_at, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+credColumns,
		userID, credType, hashedValue, label, expiresAt, metadata,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create credential: %w", err)
	}
	return c, nil
}

// GetByUserAndType returns the first credential of the given type for a user.
// Used for PASSWORD and SSO_IDENTITY lookups.
func (r *pgCredentialRepository) GetByUserAndType(ctx context.Context, userID uuid.UUID, credType model.CredentialType) (*model.Credential, error) {
	return scanCredential(r.pool.QueryRow(ctx,
		`SELECT `+credColumns+` FROM credentials
		 WHERE user_id = $1 AND type = $2
		 LIMIT 1`,
		userID, credType,
	))
}

func (r *pgCredentialRepository) GetByID(ctx context.Context, credID uuid.UUID) (*model.Credential, error) {
	return scanCredential(r.pool.QueryRow(ctx,
		`SELECT `+credColumns+` FROM credentials WHERE id = $1`,
		credID,
	))
}

// ListByUserAndType lists all credentials of a given type for a user.
// Used for API token listing.
func (r *pgCredentialRepository) ListByUserAndType(ctx context.Context, userID uuid.UUID, credType model.CredentialType) ([]*model.Credential, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+credColumns+` FROM credentials
		 WHERE user_id = $1 AND type = $2
		 ORDER BY created_at DESC`,
		userID, credType,
	)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var result []*model.Credential
	for rows.Next() {
		c := &model.Credential{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan credential row: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// Delete hard-deletes a credential by ID, scoped to the owning user.
func (r *pgCredentialRepository) Delete(ctx context.Context, credID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM credentials WHERE id = $1 AND user_id = $2`,
		credID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
