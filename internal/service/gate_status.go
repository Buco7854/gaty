package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/redis/go-redis/v9"
)

// gateEventChannel is the Redis Pub/Sub channel for gate status events.
const gateEventChannel = "gate:events"

// GateStatusEvent is the Redis Pub/Sub payload published on every gate status change.
type GateStatusEvent struct {
	GateID         string         `json:"gate_id"`
	Status         string         `json:"status"`
	StatusMetadata map[string]any `json:"status_metadata,omitempty"`
}

// ProcessGateStatus is the single entry point for all gate status updates.
func ProcessGateStatus(
	ctx context.Context,
	gateRepo repository.GateRepository,
	redisClient *redis.Client,
	gate *model.Gate,
	rawStatus string,
	meta map[string]any,
) error {
	finalStatus := rawStatus
	if override, ok := model.EvaluateStatusRules(gate.StatusRules, meta); ok {
		finalStatus = override
	}

	if err := gateRepo.UpdateStatus(ctx, gate.ID, finalStatus, meta); err != nil {
		return fmt.Errorf("persist gate status: %w", err)
	}

	if redisClient != nil {
		publishGateStatusEvent(ctx, redisClient, GateStatusEvent{
			GateID:         gate.ID.String(),
			Status:         finalStatus,
			StatusMetadata: meta,
		})
	}
	return nil
}

// publishGateStatusEvent marshals ev and publishes it to the Redis channel.
func publishGateStatusEvent(ctx context.Context, redisClient *redis.Client, ev GateStatusEvent) {
	payload, _ := json.Marshal(ev)
	tCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Publish(tCtx, gateEventChannel, string(payload)).Err(); err != nil {
		slog.Warn("gate: failed to publish status event", "channel", gateEventChannel, "error", err)
	}
}
