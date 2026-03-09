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
// Format: workspace_{wsID}/gates/{gateID}/status
func StatusTopic(wsID, gateID uuid.UUID) string {
	return fmt.Sprintf("workspace_%s/gates/%s/status", wsID, gateID)
}

// CommandTopic returns the MQTT topic to send commands to a gate.
// Format: workspace_{wsID}/gates/{gateID}/command
func CommandTopic(wsID, gateID uuid.UUID) string {
	return fmt.Sprintf("workspace_%s/gates/%s/command", wsID, gateID)
}

// SubscribeGateStatuses subscribes to all gate status topics and delegates
// status processing to service.ProcessGateStatus (rule evaluation, DB
// persistence, SSE pub/sub).
// Topic wildcard: +/gates/+/status
//
// The payload "token" field (gate JWT) is always required and verified
// for signature + topic consistency (gate_id in token must match topic).
// The auth mode only controls whether the token is also checked against
// the DB for rotation:
//
//   - brokerAuth=true  (MQTT_AUTH_MODE=dynsec): JWT signature + topic
//     match only. The broker already validated credentials at CONNECT
//     time via the Dynamic Security Plugin, so DB lookup is skipped.
//
//   - brokerAuth=false (MQTT_AUTH_MODE=payload): full validation — JWT
//     signature, topic match, AND DB lookup (GetByToken) to reject
//     rotated tokens. This is the only line of defense when the broker
//     accepts anonymous connections.
//
// ── Migration note ──────────────────────────────────────────────────
// These two branches are the critical switch point for broker auth.
// If the Dynamic Security Plugin (or any future broker auth backend)
// becomes unavailable, set MQTT_AUTH_MODE=payload to fall back to
// full app-level token validation here. No other code changes needed.
func (c *Client) SubscribeGateStatuses(gateRepo repository.GateRepository, redisClient *redis.Client, brokerAuth bool, jwtSecret []byte) error {
	return c.Subscribe("+/gates/+/status", func(_ pahomqtt.Client, msg pahomqtt.Message) {
		_, topicGateID, err := parseTopicIDs(msg.Topic())
		if err != nil {
			slog.Warn("mqtt: cannot parse ids from topic", "topic", msg.Topic())
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
		claimedGateID, _, err := service.ParseGateToken(token, jwtSecret)
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
			// ── BROKER AUTH PATH (dynsec) ────────────────────────────────
			// Broker validated credentials at CONNECT. Token signature and
			// topic match are verified above; skip DB rotation check.
			gate, err = gateRepo.GetByIDPublic(ctx, topicGateID)
			if err != nil {
				slog.Warn("mqtt: gate not found", "gate_id", topicGateID, "error", err)
				return
			}
		} else {
			// ── APP-LEVEL AUTH PATH (payload) ────────────────────────────
			// No broker auth — also verify the token against DB to reject
			// rotated tokens (full validation).
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
// Handles both MQTT_GATIE (fixed format, meta extracted by MetaConfig keys) and MQTT_CUSTOM (user mapping).
func resolveMQTTPayload(gate *model.Gate, raw map[string]any) (status string, meta map[string]any, err error) {
	if gate.StatusConfig == nil {
		return "", nil, fmt.Errorf("no MQTT status_config configured")
	}
	switch gate.StatusConfig.Type {
	case model.DriverTypeMQTTGatie:
		// Native Gaty protocol: {"token":"...","status":"...",...}
		// Meta extracted via gate.MetaConfig keys from root.
		s, _ := raw["status"].(string)
		if s == "" {
			return "", nil, fmt.Errorf("mqtt_gatie: missing or empty 'status' field in payload")
		}
		return s, model.ExtractMeta(gate.MetaConfig, raw), nil
	case model.DriverTypeMQTTCustom:
		// MQTT_CUSTOM: status extracted via PayloadMapping; meta via gate.MetaConfig.
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
