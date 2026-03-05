package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/redis/go-redis/v9"
)

// GateTTLWorker periodically marks gates as unresponsive when they haven't
// sent a status update (or keepalive ping) within the configured TTL.
//
// Both MQTT and HTTP-mode gates update last_seen_at on every message/ping.
// If no update arrives within TTL, the gate is marked "unresponsive" and a
// real-time SSE event is published so connected clients update immediately.
type GateTTLWorker struct {
	gates *repository.GateRepository
	redis *redis.Client
}

// sseGateEvent mirrors internalmqtt.GateEvent to avoid circular imports.
type sseGateEvent struct {
	GateID      string `json:"gate_id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
}

func NewGateTTLWorker(gates *repository.GateRepository, redis *redis.Client) *GateTTLWorker {
	return &GateTTLWorker{gates: gates, redis: redis}
}

// Run starts the TTL check loop. It ticks every interval and marks gates
// unresponsive when their last_seen_at is older than ttl.
// The loop stops when ctx is cancelled.
func (w *GateTTLWorker) Run(ctx context.Context, interval, ttl time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("gate TTL worker started", "interval", interval, "ttl", ttl)

	for {
		select {
		case <-ticker.C:
			w.tick(ctx, ttl)
		case <-ctx.Done():
			slog.Info("gate TTL worker stopped")
			return
		}
	}
}

func (w *GateTTLWorker) tick(ctx context.Context, ttl time.Duration) {
	tCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	affected, err := w.gates.MarkUnresponsiveWithIDs(tCtx, ttl)
	if err != nil {
		slog.Error("gate TTL: failed to mark unresponsive gates", "error", err)
		return
	}
	if len(affected) == 0 {
		return
	}

	slog.Info("gate TTL: marked gates unresponsive", "count", len(affected))

	if w.redis == nil {
		return
	}

	for _, g := range affected {
		event := sseGateEvent{
			GateID:      g.GateID.String(),
			WorkspaceID: g.WorkspaceID.String(),
			Status:      string(model.GateStatusUnresponsive),
		}
		payload, _ := json.Marshal(event)
		channel := fmt.Sprintf("gate:events:%s", g.WorkspaceID)
		if err := w.redis.Publish(tCtx, channel, string(payload)).Err(); err != nil {
			slog.Warn("gate TTL: failed to publish unresponsive event",
				"gate_id", g.GateID, "error", err)
		}
	}
}
