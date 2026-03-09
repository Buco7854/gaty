package service

import (
	"context"
	"fmt"
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
// sent a status update within the configured TTL. It also applies automatic
// status transitions when configured.
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
			w.checkTransitions(ctx)
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

// transitionRedisKey returns the Redis key used to track when a transition was armed.
func transitionRedisKey(gateID, from, to string) string {
	return fmt.Sprintf("gate:tr:%s:%s:%s", gateID, from, to)
}

// checkTransitions applies automatic status transitions for gates whose current
// status matches a transition's "from" and whose deadline has elapsed.
//
// Behavior depends on the transition's OnNewStatus field:
//   - "reset" (default): deadline = last_seen_at + after_seconds. Timer resets
//     naturally because last_seen_at updates on every status message.
//   - "cancel": transition arms once. If a new status message arrives (last_seen_at
//     changes), the transition is cancelled and won't re-arm until the gate leaves
//     the "from" status and comes back.
//   - "continue": transition arms once. Deadline uses the original armed time,
//     ignoring subsequent status messages.
func (w *GateTTLWorker) checkTransitions(ctx context.Context) {
	tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	gates, err := w.gates.ListTransitionCandidates(tCtx)
	if err != nil {
		slog.Error("gate transitions: failed to list candidates", "error", err)
		return
	}

	now := time.Now()
	for i := range gates {
		g := &gates[i]
		if g.LastSeenAt == nil {
			continue
		}
		for _, t := range g.StatusTransitions {
			rKey := transitionRedisKey(g.ID.String(), t.From, t.To)

			if string(g.Status) != t.From {
				// Status no longer matches — clean up armed key so it can
				// re-arm when the gate returns to this status.
				if w.redis != nil && t.OnNewStatus != "" && t.OnNewStatus != model.TransitionReset {
					w.redis.Del(tCtx, rKey)
				}
				continue
			}

			afterDur := time.Duration(t.AfterSeconds) * time.Second

			switch t.OnNewStatus {
			case model.TransitionCancel:
				if w.redis == nil {
					// Fallback to reset behavior without Redis.
					if now.Before(g.LastSeenAt.Add(afterDur)) {
						continue
					}
				} else {
					armedMs, err := w.redis.Get(tCtx, rKey).Int64()
					if err != nil {
						// Not armed yet — arm now with current last_seen_at.
						w.redis.Set(tCtx, rKey, g.LastSeenAt.UnixMicro(), 0)
						continue
					}
					armedAt := time.UnixMicro(armedMs)
					if !g.LastSeenAt.Equal(armedAt) {
						// A new status message arrived since arming → cancelled.
						// Keep the key so we don't re-arm; it's cleaned when
						// status leaves "from".
						continue
					}
					if now.Before(armedAt.Add(afterDur)) {
						continue
					}
					w.redis.Del(tCtx, rKey)
				}

			case model.TransitionContinue:
				if w.redis == nil {
					if now.Before(g.LastSeenAt.Add(afterDur)) {
						continue
					}
				} else {
					armedMs, err := w.redis.Get(tCtx, rKey).Int64()
					if err != nil {
						// Not armed yet — arm now.
						w.redis.Set(tCtx, rKey, g.LastSeenAt.UnixMicro(), 0)
						continue
					}
					armedAt := time.UnixMicro(armedMs)
					if now.Before(armedAt.Add(afterDur)) {
						continue
					}
					w.redis.Del(tCtx, rKey)
				}

			default: // "reset" or empty — current behavior
				if now.Before(g.LastSeenAt.Add(afterDur)) {
					continue
				}
			}

			// Transition is due — apply it directly (no status rules evaluation).
			if err := w.gates.UpdateStatus(tCtx, g.ID, t.To, g.StatusMetadata); err != nil {
				slog.Error("gate transitions: failed to update status",
					"gate_id", g.ID, "from", t.From, "to", t.To, "error", err)
				continue
			}
			slog.Info("gate transitions: applied",
				"gate_id", g.ID, "from", t.From, "to", t.To,
				"after_seconds", t.AfterSeconds, "on_new_status", t.OnNewStatus)

			if w.redis != nil {
				publishGateStatusEvent(tCtx, w.redis, GateStatusEvent{
					GateID:      g.ID.String(),
					WorkspaceID: g.WorkspaceID.String(),
					Status:      t.To,
				})
			}
			break // first matching transition wins
		}
	}
}
