package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	// Production-grade pool configuration.
	// These defaults balance throughput and Postgres connection limits.
	// Override via DATABASE_URL query params (?pool_max_conns=...) if needed.
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute  // recycle long-lived connections
	cfg.MaxConnIdleTime = 10 * time.Minute  // release idle connections quickly
	cfg.HealthCheckPeriod = 1 * time.Minute // proactive keepalive checks

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}
