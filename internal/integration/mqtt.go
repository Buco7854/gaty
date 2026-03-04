package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
)

// MQTTDriver publishes a command to the gate's MQTT command topic.
type MQTTDriver struct {
	client *internalmqtt.Client
	action string // e.g. "open", "close"
}

func (d *MQTTDriver) Execute(_ context.Context, gate *model.Gate) error {
	payload, err := json.Marshal(map[string]string{"action": d.action})
	if err != nil {
		return fmt.Errorf("mqtt driver: marshal payload: %w", err)
	}
	topic := internalmqtt.CommandTopic(gate.WorkspaceID, gate.ID)
	if err := d.client.Publish(topic, payload); err != nil {
		return fmt.Errorf("mqtt driver: publish: %w", err)
	}
	return nil
}
