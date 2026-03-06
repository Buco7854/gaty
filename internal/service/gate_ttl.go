package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultGateTTL is the inactivity threshold after which the TTL worker marks a gate
	// "unresponsive" in the database. Must be less than model.OfflineThreshold (5 min),
	// which is when the API layer begins returning "offline" based on last_seen_at.
	DefaultGateTTL = 30 * time.Second

	// ttlCheckInterval is how often the worker scans for expired gates.
	// Short enough to catch expiries within a few seconds without hammering the DB.
	// The partial index on (last_seen_at) makes each scan very cheap.
	ttlCheckInterval = 5 * time.Second
)

// GateTTLWorker periodically marks gates as "unresponsive" when they haven't
// sent a status update within the configured TTL.
//
// Architecture note: the worker relies on the partial index
// idx_gates_ttl_candidates so the bulk UPDATE touches only candidate rows,
// keeping the query O(k) where k is the number of newly-expired gates
// (typically zero between ticks).
type GateTTLWorker struct {
	gates repository.GateRepository
	redis *redis.Client
	ttl   time.Duration
}

// NewGateTTLWorker creates a worker with the given inactivity threshold.
func NewGateTTLWorker(gates repository.GateRepository, redis *redis.Client, ttl time.Duration) *GateTTLWorker {
	return &GateTTLWorker{gates: gates, redis: redis, ttl: ttl}
}

// Run starts the TTL check loop. It blocks until ctx is cancelled.
func (w *GateTTLWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(ttlCheckInterval)
	defer ticker.Stop()

	slog.Info("gate TTL worker started", "ttl", w.ttl, "check_interval", ttlCheckInterval)

	for {
		select {
		case <-ticker.C:
			w.markExpired(ctx)
		case <-ctx.Done():
			slog.Info("gate TTL worker stopped")
			return
		}
	}
}

func (w *GateTTLWorker) markExpired(ctx context.Context) {
	tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	affected, err := w.gates.MarkUnresponsiveWithIDs(tCtx, w.ttl)
	if err != nil {
		slog.Error("gate TTL: failed to mark unresponsive gates", "error", err)
		return
	}
	if len(affected) == 0 || w.redis == nil {
		return
	}

	slog.Info("gate TTL: gates marked unresponsive", "count", len(affected))

	for _, g := range affected {
		publishGateStatusEvent(tCtx, w.redis, GateStatusEvent{
			GateID:      g.GateID.String(),
			WorkspaceID: g.WorkspaceID.String(),
			Status:      string(model.GateStatusUnresponsive),
		})
	}
}
