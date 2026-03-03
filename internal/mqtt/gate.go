package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

type statusPayload struct {
	Status string `json:"status"`
}

// GateEvent is published to Redis Pub/Sub when a gate status changes.
type GateEvent struct {
	GateID      string `json:"gate_id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
}

// SubscribeGateStatuses subscribes to all gate status topics, updates the DB,
// and if redisClient is non-nil, publishes GateEvents to gate:events:{workspace_id}.
// Topic wildcard: +/gates/+/status
func (c *Client) SubscribeGateStatuses(gateRepo *repository.GateRepository, redisClient *redis.Client) error {
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

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := gateRepo.UpdateStatus(ctx, gateID, p.Status); err != nil {
			slog.Error("mqtt: failed to update gate status", "gate_id", gateID, "error", err)
			return
		}

		if redisClient != nil {
			event := GateEvent{
				GateID:      gateID.String(),
				WorkspaceID: wsID.String(),
				Status:      p.Status,
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
