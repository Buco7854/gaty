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
	DefaultGateTTL   = 30 * time.Second
	ttlCheckInterval = 5 * time.Second
)

// GateTTLWorker periodically marks gates as "unresponsive" and applies status transitions.
type GateTTLWorker struct {
	gates repository.GateRepository
	redis *redis.Client
	ttl   time.Duration
}

func NewGateTTLWorker(gates repository.GateRepository, redis *redis.Client, ttl time.Duration) *GateTTLWorker {
	return &GateTTLWorker{gates: gates, redis: redis, ttl: ttl}
}

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
			GateID: g.GateID.String(),
			Status: string(model.GateStatusUnresponsive),
		})
	}
}

func transitionRedisKey(gateID, from, to string) string {
	return fmt.Sprintf("gate:tr:%s:%s:%s", gateID, from, to)
}

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
			afterDur := time.Duration(t.AfterSeconds) * time.Second
			statusMatches := string(g.Status) == t.From

			if !statusMatches {
				if !t.PersistOnChange {
					if w.redis != nil {
						w.redis.Del(tCtx, rKey)
					}
					continue
				}

				if w.redis == nil {
					continue
				}
				armedMs, err := w.redis.Get(tCtx, rKey).Int64()
				if err != nil {
					continue
				}
				armedAt := time.UnixMicro(armedMs)
				if now.Before(armedAt.Add(afterDur)) {
					continue
				}
				w.redis.Del(tCtx, rKey)
			} else {
				if t.PersistOnChange && w.redis != nil {
					w.redis.Set(tCtx, rKey, g.LastSeenAt.UnixMicro(), 0)
				}
				if now.Before(g.LastSeenAt.Add(afterDur)) {
					continue
				}
				if w.redis != nil {
					w.redis.Del(tCtx, rKey)
				}
			}

			if err := w.gates.UpdateStatus(tCtx, g.ID, t.To, g.StatusMetadata); err != nil {
				slog.Error("gate transitions: failed to update status",
					"gate_id", g.ID, "from", t.From, "to", t.To, "error", err)
				continue
			}
			slog.Info("gate transitions: applied",
				"gate_id", g.ID, "from", t.From, "to", t.To,
				"after_seconds", t.AfterSeconds,
				"persist_on_change", t.PersistOnChange)

			if w.redis != nil {
				publishGateStatusEvent(tCtx, w.redis, GateStatusEvent{
					GateID: g.ID.String(),
					Status: t.To,
				})
			}
			break
		}
	}
}
