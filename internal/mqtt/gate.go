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

// SubscribeGateStatuses subscribes to all gate status topics and updates the DB accordingly.
// Topic wildcard: +/gates/+/status
func (c *Client) SubscribeGateStatuses(gateRepo *repository.GateRepository) error {
	return c.Subscribe("+/gates/+/status", func(_ pahomqtt.Client, msg pahomqtt.Message) {
		gateID, err := parseGateIDFromTopic(msg.Topic())
		if err != nil {
			slog.Warn("mqtt: cannot parse gate id from topic", "topic", msg.Topic())
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
		}
	})
}

// parseGateIDFromTopic extracts the gate UUID from topics like
// "workspace_{wsID}/gates/{gateID}/status".
func parseGateIDFromTopic(topic string) (uuid.UUID, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 {
		return uuid.Nil, fmt.Errorf("unexpected topic format: %s", topic)
	}
	return uuid.Parse(parts[2])
}
