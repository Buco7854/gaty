package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	internalmqtt "github.com/Buco7854/gatie/internal/mqtt"
)

// MQTTGatieDriver publishes a Gaty-protocol command to the gate's MQTT command topic.
// Payload: {"action": "open|close"}
type MQTTGatieDriver struct {
	client *internalmqtt.Client
	action string // "open" or "close"
}

func (d *MQTTGatieDriver) Execute(_ context.Context, gate *model.Gate) error {
	payload, err := json.Marshal(map[string]string{"action": d.action})
	if err != nil {
		return fmt.Errorf("mqtt_gatie driver: marshal payload: %w", err)
	}
	topic := internalmqtt.CommandTopic(gate.WorkspaceID, gate.ID)
	if err := d.client.Publish(topic, payload); err != nil {
		return fmt.Errorf("mqtt_gatie driver: publish: %w", err)
	}
	return nil
}

// MQTTCustomDriver publishes a user-defined static JSON payload to the gate's MQTT command topic.
// Payload: config["payload"] marshaled as-is.
type MQTTCustomDriver struct {
	client  *internalmqtt.Client
	payload map[string]any
}

func (d *MQTTCustomDriver) Execute(_ context.Context, gate *model.Gate) error {
	payload, err := json.Marshal(d.payload)
	if err != nil {
		return fmt.Errorf("mqtt_custom driver: marshal payload: %w", err)
	}
	topic := internalmqtt.CommandTopic(gate.WorkspaceID, gate.ID)
	if err := d.client.Publish(topic, payload); err != nil {
		return fmt.Errorf("mqtt_custom driver: publish: %w", err)
	}
	return nil
}
