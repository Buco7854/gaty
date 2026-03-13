package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StatusTopic returns the MQTT topic for a gate's status updates.
// Format: gates/{gateID}/status
func StatusTopic(gateID uuid.UUID) string {
	return fmt.Sprintf("gates/%s/status", gateID)
}

// CommandTopic returns the MQTT topic to send commands to a gate.
// Format: gates/{gateID}/command
func CommandTopic(gateID uuid.UUID) string {
	return fmt.Sprintf("gates/%s/command", gateID)
}

// SubscribeGateStatuses subscribes to all gate status topics and delegates
// status processing to service.ProcessGateStatus (rule evaluation, DB
// persistence, SSE pub/sub).
// Topic wildcard: gates/+/status
func (c *Client) SubscribeGateStatuses(gateRepo repository.GateRepository, redisClient *redis.Client, brokerAuth bool, jwtSecret []byte) error {
	return c.Subscribe("gates/+/status", func(_ pahomqtt.Client, msg pahomqtt.Message) {
		topicGateID, err := parseTopicGateID(msg.Topic())
		if err != nil {
			slog.Warn("mqtt: cannot parse gate id from topic", "topic", msg.Topic())
			return
		}

		var raw map[string]any
		if err := json.Unmarshal(msg.Payload(), &raw); err != nil {
			slog.Warn("mqtt: invalid JSON payload", "topic", msg.Topic())
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// ── Token is always required and signature-checked ──────────
		token, _ := raw["token"].(string)
		if token == "" {
			slog.Warn("mqtt: status payload missing token", "gate_id", topicGateID)
			return
		}
		claimedGateID, err := service.ParseGateToken(token, jwtSecret)
		if err != nil {
			slog.Warn("mqtt: invalid gate token signature", "gate_id", topicGateID)
			return
		}
		if claimedGateID != topicGateID {
			slog.Warn("mqtt: token gate_id mismatch with topic", "topic_gate_id", topicGateID, "token_gate_id", claimedGateID)
			return
		}

		var gate *model.Gate
		if brokerAuth {
			gate, err = gateRepo.GetByIDPublic(ctx, topicGateID)
			if err != nil {
				slog.Warn("mqtt: gate not found", "gate_id", topicGateID, "error", err)
				return
			}
		} else {
			gate, err = gateRepo.GetByToken(ctx, topicGateID, token)
			if err != nil {
				slog.Warn("mqtt: gate token invalid or rotated", "gate_id", topicGateID)
				return
			}
		}

		status, meta, err := resolveMQTTPayload(gate, raw)
		if err != nil {
			slog.Warn("mqtt: payload mapping failed", "gate_id", topicGateID, "error", err)
			return
		}

		if err := service.ProcessGateStatus(ctx, gateRepo, redisClient, gate, status, meta); err != nil {
			slog.Error("mqtt: failed to process gate status", "gate_id", topicGateID, "error", err)
		}
	})
}

// resolveMQTTPayload extracts status and metadata from a raw MQTT JSON payload.
func resolveMQTTPayload(gate *model.Gate, raw map[string]any) (status string, meta map[string]any, err error) {
	if gate.StatusConfig == nil {
		return "", nil, fmt.Errorf("no MQTT status_config configured")
	}
	switch gate.StatusConfig.Type {
	case model.DriverTypeMQTTGatie:
		s, _ := raw["status"].(string)
		if s == "" {
			return "", nil, fmt.Errorf("mqtt_gatie: missing or empty 'status' field in payload")
		}
		return s, model.ExtractMeta(gate.MetaConfig, raw), nil
	case model.DriverTypeMQTTCustom:
		if len(gate.StatusConfig.Config) == 0 {
			return "", nil, fmt.Errorf("no MQTT_CUSTOM status_config mapping configured")
		}
		mapping, ok := model.ExtractPayloadMapping(gate.StatusConfig.Config)
		if !ok {
			return "", nil, fmt.Errorf("status_config.config.mapping missing or invalid")
		}
		status, err := model.ApplyMapping(*mapping, raw)
		if err != nil {
			return "", nil, err
		}
		return status, model.ExtractMeta(gate.MetaConfig, raw), nil
	default:
		return "", nil, fmt.Errorf("unsupported MQTT status_config type: %q", gate.StatusConfig.Type)
	}
}

// parseTopicGateID extracts gate UUID from topics like "gates/{gateID}/status".
func parseTopicGateID(topic string) (uuid.UUID, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "gates" {
		return uuid.Nil, fmt.Errorf("unexpected topic format: %s", topic)
	}
	gateID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse gate id: %w", err)
	}
	return gateID, nil
}
