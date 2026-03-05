package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StatusTopic returns the MQTT topic for a gate's status updates.
// Format: workspace_{wsID}/gates/{gateID}/status
func StatusTopic(wsID, gateID uuid.UUID) string {
	return fmt.Sprintf("workspace_%s/gates/%s/status", wsID, gateID)
}

// CommandTopic returns the MQTT topic to send commands to a gate.
// Format: workspace_{wsID}/gates/{gateID}/command
func CommandTopic(wsID, gateID uuid.UUID) string {
	return fmt.Sprintf("workspace_%s/gates/%s/command", wsID, gateID)
}

// statusPayload is the message a gate publishes on its status topic.
//
// Required fields:
//   - token  — the gate's authentication secret (gate_token column)
//   - status — system-level state string (e.g. "open", "closed", "online", "offline")
//
// Optional field:
//   - meta — arbitrary metadata (e.g. {"lora.snr": -10.5, "battery": 85})
type statusPayload struct {
	Token  string         `json:"token"`
	Status string         `json:"status"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// GateEvent is published to Redis Pub/Sub when a gate status changes.
type GateEvent struct {
	GateID         string         `json:"gate_id"`
	WorkspaceID    string         `json:"workspace_id"`
	Status         string         `json:"status"`
	StatusMetadata map[string]any `json:"status_metadata,omitempty"`
}

// SubscribeGateStatuses subscribes to all gate status topics, authenticates the gate,
// applies status rules (business logic), persists the result, and publishes SSE events.
// Topic wildcard: +/gates/+/status
func (c *Client) SubscribeGateStatuses(gateRepo repository.GateRepository, redisClient *redis.Client) error {
	return c.Subscribe("+/gates/+/status", func(_ pahomqtt.Client, msg pahomqtt.Message) {
		wsID, gateID, err := parseTopicIDs(msg.Topic())
		if err != nil {
			slog.Warn("mqtt: cannot parse ids from topic", "topic", msg.Topic())
			return
		}

		var p statusPayload
		if err := json.Unmarshal(msg.Payload(), &p); err != nil {
			slog.Warn("mqtt: invalid status payload", "topic", msg.Topic())
			return
		}
		if p.Token == "" {
			slog.Warn("mqtt: status payload missing token", "gate_id", gateID)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Validate token and retrieve the gate (with status_rules).
		gate, err := gateRepo.GetByToken(ctx, gateID, p.Token)
		if err != nil {
			if err == repository.ErrUnauthorized {
				slog.Warn("mqtt: invalid gate token, status update rejected", "gate_id", gateID)
			} else {
				slog.Error("mqtt: failed to authenticate gate", "gate_id", gateID, "error", err)
			}
			return
		}

		// Business logic: evaluate status rules against the incoming metadata.
		finalStatus := p.Status
		if override, ok := model.EvaluateStatusRules(gate.StatusRules, p.Meta); ok {
			finalStatus = override
		}

		if err := gateRepo.UpdateStatus(ctx, gateID, finalStatus, p.Meta); err != nil {
			slog.Error("mqtt: failed to update gate status", "gate_id", gateID, "error", err)
			return
		}

		if redisClient != nil {
			event := GateEvent{
				GateID:         gateID.String(),
				WorkspaceID:    wsID.String(),
				Status:         finalStatus,
				StatusMetadata: p.Meta,
			}
			payload, _ := json.Marshal(event)
			channel := fmt.Sprintf("gate:events:%s", wsID)
			if err := redisClient.Publish(ctx, channel, string(payload)).Err(); err != nil {
				slog.Warn("mqtt: failed to publish gate event", "channel", channel, "error", err)
			}
		}
	})
}

// parseTopicIDs extracts workspace UUID and gate UUID from topics like
// "workspace_{wsID}/gates/{gateID}/status".
func parseTopicIDs(topic string) (wsID, gateID uuid.UUID, err error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 {
		return uuid.Nil, uuid.Nil, fmt.Errorf("unexpected topic format: %s", topic)
	}
	wsPrefix := parts[0]
	if !strings.HasPrefix(wsPrefix, "workspace_") {
		return uuid.Nil, uuid.Nil, fmt.Errorf("unexpected workspace prefix: %s", wsPrefix)
	}
	wsID, err = uuid.Parse(strings.TrimPrefix(wsPrefix, "workspace_"))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse workspace id: %w", err)
	}
	gateID, err = uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse gate id: %w", err)
	}
	return wsID, gateID, nil
}
