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
// The SSE handler subscribes to this channel to fan-out updates to browsers.
const gateEventChannel = "gate:events:%s" // formatted with workspace UUID

// GateStatusEvent is the Redis Pub/Sub payload published on every gate status change.
// Consumed by the SSE fan-out handler (handler/sse.go).
type GateStatusEvent struct {
	GateID         string         `json:"gate_id"`
	WorkspaceID    string         `json:"workspace_id"`
	Status         string         `json:"status"`
	StatusMetadata map[string]any `json:"status_metadata,omitempty"`
}

// ProcessGateStatus is the single entry point for all gate status updates,
// regardless of transport (MQTT or HTTP inbound).
//
// It evaluates status rules against the incoming metadata, persists the
// resolved status, and publishes a GateStatusEvent for SSE fan-out.
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
			WorkspaceID:    gate.WorkspaceID.String(),
			Status:         finalStatus,
			StatusMetadata: meta,
		})
	}
	return nil
}

// publishGateStatusEvent marshals ev and publishes it to the workspace Redis channel.
// Errors are logged as warnings; the caller is not blocked.
func publishGateStatusEvent(ctx context.Context, redisClient *redis.Client, ev GateStatusEvent) {
	payload, _ := json.Marshal(ev)
	channel := fmt.Sprintf(gateEventChannel, ev.WorkspaceID)
	tCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := redisClient.Publish(tCtx, channel, string(payload)).Err(); err != nil {
		slog.Warn("gate: failed to publish status event", "channel", channel, "error", err)
	}
}
