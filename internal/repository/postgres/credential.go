package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type credentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) repository.CredentialRepository {
	return &credentialRepository{pool: pool}
}

const credColumns = `id, member_id, type, hashed_value, label, expires_at, metadata, created_at`

func scanCredential(row pgx.Row) (*model.Credential, error) {
	c := &model.Credential{}
	err := row.Scan(&c.ID, &c.MemberID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan credential: %w", err)
	}
	return c, nil
}

func (r *credentialRepository) Create(ctx context.Context, memberID uuid.UUID, credType model.CredentialType, hashedValue string, label *string, expiresAt *time.Time, metadata map[string]any) (*model.Credential, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	c, err := scanCredential(r.pool.QueryRow(ctx,
		`INSERT INTO credentials (member_id, type, hashed_value, label, expires_at, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+credColumns,
		memberID, credType, hashedValue, label, expiresAt, metadata,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("create credential: %w", err)
	}
	return c, nil
}

func (r *credentialRepository) GetByMemberAndType(ctx context.Context, memberID uuid.UUID, credType model.CredentialType) (*model.Credential, error) {
	return scanCredential(r.pool.QueryRow(ctx,
		`SELECT `+credColumns+` FROM credentials
		 WHERE member_id = $1 AND type = $2
		 LIMIT 1`,
		memberID, credType,
	))
}

func (r *credentialRepository) GetByID(ctx context.Context, credID, memberID uuid.UUID) (*model.Credential, error) {
	return scanCredential(r.pool.QueryRow(ctx,
		`SELECT `+credColumns+` FROM credentials WHERE id = $1 AND member_id = $2`,
		credID, memberID,
	))
}

func (r *credentialRepository) ListByMemberAndType(ctx context.Context, memberID uuid.UUID, credType model.CredentialType, p model.PaginationParams) ([]*model.Credential, int, error) {
	p = p.Normalize()

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM credentials WHERE member_id = $1 AND type = $2`,
		memberID, credType,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count credentials: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+credColumns+` FROM credentials
		 WHERE member_id = $1 AND type = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		memberID, credType, p.Limit, p.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var result []*model.Credential
	for rows.Next() {
		c := &model.Credential{}
		if err := rows.Scan(&c.ID, &c.MemberID, &c.Type, &c.HashedValue, &c.Label, &c.ExpiresAt, &c.Metadata, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan credential row: %w", err)
		}
		result = append(result, c)
	}
	return result, total, rows.Err()
}

func (r *credentialRepository) FindBySSOIdentity(ctx context.Context, providerSub string) (*model.Credential, error) {
	return scanCredential(r.pool.QueryRow(ctx,
		`SELECT `+credColumns+` FROM credentials
		 WHERE type = $1 AND hashed_value = $2
		 LIMIT 1`,
		model.CredSSOIdentity, providerSub,
	))
}

func (r *credentialRepository) FindByHashedAPIToken(ctx context.Context, hash string) (*model.Credential, *model.Member, error) {
	cred := &model.Credential{}
	member := &model.Member{}
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.member_id, c.type, c.hashed_value, c.label, c.expires_at, c.metadata, c.created_at,
		        m.id, m.username, m.display_name, m.role, m.created_at
		 FROM credentials c
		 JOIN members m ON m.id = c.member_id
		 WHERE c.hashed_value = $1 AND c.type = 'API_TOKEN'
		   AND (c.expires_at IS NULL OR c.expires_at > NOW())`,
		hash,
	).Scan(
		&cred.ID, &cred.MemberID, &cred.Type, &cred.HashedValue, &cred.Label, &cred.ExpiresAt, &cred.Metadata, &cred.CreatedAt,
		&member.ID, &member.Username, &member.DisplayName, &member.Role, &member.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("find cred by api token: %w", err)
	}
	return cred, member, nil
}

func (r *credentialRepository) UpdateHashedValue(ctx context.Context, memberID uuid.UUID, credType model.CredentialType, newHashedValue string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE credentials SET hashed_value = $1 WHERE member_id = $2 AND type = $3`,
		newHashedValue, memberID, credType,
	)
	if err != nil {
		return fmt.Errorf("update credential hashed value: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *credentialRepository) Delete(ctx context.Context, credID, memberID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM credentials WHERE id = $1 AND member_id = $2`,
		credID, memberID,
	)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}
