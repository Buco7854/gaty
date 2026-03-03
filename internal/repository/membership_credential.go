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

type MembershipCredentialRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipCredentialRepository(pool *pgxpool.Pool) *MembershipCredentialRepository {
	return &MembershipCredentialRepository{pool: pool}
}

const membershipCredColumns = `id, membership_id, type, hashed_value, label, expires_at, metadata, created_at`

func scanMembershipCredential(row pgx.Row) (*model.MembershipCredential, error) {
	c := &model.MembershipCredential{}
	err := row.Scan(&c.ID, &c.MembershipID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan membership credential: %w", err)
	}
	return c, nil
}

func (r *MembershipCredentialRepository) Create(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.MembershipCredential, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	c, err := scanMembershipCredential(r.pool.QueryRow(ctx,
		`INSERT INTO membership_credentials (membership_id, type, hashed_value, label, expires_at, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+membershipCredColumns,
		membershipID, credType, hashedValue, label, expiresAt, metadata,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("create membership credential: %w", err)
	}
	return c, nil
}

// GetByMembershipAndType returns the first credential of a given type for a membership.
// Used for PASSWORD and SSO_IDENTITY lookups.
func (r *MembershipCredentialRepository) GetByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) (*model.MembershipCredential, error) {
	return scanMembershipCredential(r.pool.QueryRow(ctx,
		`SELECT `+membershipCredColumns+` FROM membership_credentials
		 WHERE membership_id = $1 AND type = $2
		 LIMIT 1`,
		membershipID, credType,
	))
}

func (r *MembershipCredentialRepository) GetByID(ctx context.Context, credID, membershipID uuid.UUID) (*model.MembershipCredential, error) {
	return scanMembershipCredential(r.pool.QueryRow(ctx,
		`SELECT `+membershipCredColumns+` FROM membership_credentials
		 WHERE id = $1 AND membership_id = $2`,
		credID, membershipID,
	))
}

// ListByMembershipAndType lists all credentials of a given type for a membership.
// Used for API token listing.
func (r *MembershipCredentialRepository) ListByMembershipAndType(ctx context.Context, membershipID uuid.UUID, credType model.CredentialType) ([]*model.MembershipCredential, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+membershipCredColumns+` FROM membership_credentials
		 WHERE membership_id = $1 AND type = $2
		 ORDER BY created_at DESC`,
		membershipID, credType,
	)
	if err != nil {
		return nil, fmt.Errorf("list membership credentials: %w", err)
	}
	defer rows.Close()

	var result []*model.MembershipCredential
	for rows.Next() {
		c := &model.MembershipCredential{}
		if err := rows.Scan(&c.ID, &c.MembershipID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan membership credential row: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// FindBySSOIdentity looks up a membership credential by provider sub within a workspace.
// Used to resolve an inbound SSO callback to an existing membership.
func (r *MembershipCredentialRepository) FindBySSOIdentity(ctx context.Context, workspaceID uuid.UUID, providerSub string) (*model.MembershipCredential, error) {
	c := &model.MembershipCredential{}
	err := r.pool.QueryRow(ctx,
		`SELECT mc.id, mc.membership_id, mc.type, mc.hashed_value, mc.label, mc.expires_at, mc.metadata, mc.created_at
		 FROM membership_credentials mc
		 JOIN workspace_memberships wm ON wm.id = mc.membership_id
		 WHERE wm.workspace_id = $1 AND mc.type = $2 AND mc.hashed_value = $3
		 LIMIT 1`,
		workspaceID, model.CredSSOIdentity, providerSub,
	).Scan(&c.ID, &c.MembershipID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find sso identity: %w", err)
	}
	return c, nil
}

// Delete hard-deletes a membership credential, scoped to the owning membership.
func (r *MembershipCredentialRepository) Delete(ctx context.Context, credID, membershipID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM membership_credentials WHERE id = $1 AND membership_id = $2`,
		credID, membershipID,
	)
	if err != nil {
		return fmt.Errorf("delete membership credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
