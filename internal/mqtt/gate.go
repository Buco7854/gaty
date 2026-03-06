package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
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
//   - token  — the gate's JWT token (gate_token column)
//   - status — system-level state string (e.g. "open", "closed", "online", "offline")
//
// Optional field:
//   - meta — arbitrary metadata (e.g. {"lora.snr": -10.5, "battery": 85})
type statusPayload struct {
	Token  string         `json:"token"`
	Status string         `json:"status"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// SubscribeGateStatuses subscribes to all gate status topics, authenticates the gate,
// and delegates status processing to service.ProcessGateStatus (rule evaluation,
// DB persistence, SSE pub/sub).
// Topic wildcard: +/gates/+/status
//
// Authentication is two-step:
//  1. Verify JWT signature with jwtSecret (prevents forgery).
//  2. DB lookup by gateID + token (detects rotation — old JWTs are invalid after SetToken).
func (c *Client) SubscribeGateStatuses(gateRepo repository.GateRepository, redisClient *redis.Client, jwtSecret []byte) error {
	return c.Subscribe("+/gates/+/status", func(_ pahomqtt.Client, msg pahomqtt.Message) {
		_, topicGateID, err := parseTopicIDs(msg.Topic())
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
			slog.Warn("mqtt: status payload missing token", "gate_id", topicGateID)
			return
		}

		// Step 1: verify JWT signature and extract claimed gate identity.
		claimedGateID, _, err := service.ParseGateToken(p.Token, jwtSecret)
		if err != nil {
			slog.Warn("mqtt: invalid gate token signature, rejected", "gate_id", topicGateID)
			return
		}
		// Claimed gateID must match the MQTT topic to prevent cross-gate attacks.
		if claimedGateID != topicGateID {
			slog.Warn("mqtt: token gate_id mismatch with topic", "topic_gate_id", topicGateID, "token_gate_id", claimedGateID)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Step 2: DB lookup by gateID — confirms token is current (not rotated).
		gate, err := gateRepo.GetByToken(ctx, topicGateID, p.Token)
		if err != nil {
			if errors.Is(err, repository.ErrUnauthorized) {
				slog.Warn("mqtt: gate token rotated or invalid, rejected", "gate_id", topicGateID)
			} else {
				slog.Error("mqtt: failed to authenticate gate", "gate_id", topicGateID, "error", err)
			}
			return
		}

		if err := service.ProcessGateStatus(ctx, gateRepo, redisClient, gate, p.Status, p.Meta); err != nil {
			slog.Error("mqtt: failed to process gate status", "gate_id", topicGateID, "error", err)
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
