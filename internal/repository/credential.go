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

type CredentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) *CredentialRepository {
	return &CredentialRepository{pool: pool}
}

func (r *CredentialRepository) Create(ctx context.Context, targetType model.CredentialTargetType, targetID uuid.UUID, credType model.CredentialType, hashedValue string) (*model.Credential, error) {
	cred := &model.Credential{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO credentials (target_type, target_id, credential_type, hashed_value)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, target_type, target_id, credential_type, hashed_value, created_at`,
		targetType, targetID, credType, hashedValue,
	).Scan(&cred.ID, &cred.TargetType, &cred.TargetID, &cred.CredentialType, &cred.HashedValue, &cred.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}
	return cred, nil
}

func (r *CredentialRepository) GetByTarget(ctx context.Context, targetType model.CredentialTargetType, targetID uuid.UUID, credType model.CredentialType) (*model.Credential, error) {
	cred := &model.Credential{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, target_type, target_id, credential_type, hashed_value, metadata, created_at
		 FROM credentials
		 WHERE target_type = $1 AND target_id = $2 AND credential_type = $3
		 LIMIT 1`,
		targetType, targetID, credType,
	).Scan(&cred.ID, &cred.TargetType, &cred.TargetID, &cred.CredentialType, &cred.HashedValue, &cred.Metadata, &cred.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get credential: %w", err)
	}
	return cred, nil
}
