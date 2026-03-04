// Package integration provides driver implementations for gate actions.
// Each gate can be independently configured for status, open, and close
// operations using different drivers.
package integration

import (
	"context"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	internalmqtt "github.com/Buco7854/gaty/internal/mqtt"
)

// Driver can execute an action on a gate.
type Driver interface {
	// Execute sends the action payload to the gate.
	Execute(ctx context.Context, gate *model.Gate) error
}

// NewOpenDriver builds the driver to use for opening a gate.
// Falls back to MQTT if open_config is nil and integration_type == MQTT.
func NewOpenDriver(gate *model.Gate, mqtt *internalmqtt.Client) (Driver, error) {
	if gate.OpenConfig != nil {
		return newDriver(gate.OpenConfig, mqtt)
	}
	// Backward-compat: legacy MQTT integration type
	if gate.IntegrationType == model.IntegrationTypeMQTT && mqtt != nil {
		return &MQTTDriver{client: mqtt, action: "open"}, nil
	}
	return &NoopDriver{}, nil
}

// NewCloseDriver builds the driver for closing a gate.
func NewCloseDriver(gate *model.Gate, mqtt *internalmqtt.Client) (Driver, error) {
	if gate.CloseConfig != nil {
		return newDriver(gate.CloseConfig, mqtt)
	}
	return &NoopDriver{}, nil
}

func newDriver(cfg *model.ActionConfig, mqtt *internalmqtt.Client) (Driver, error) {
	switch cfg.Type {
	case model.DriverTypeMQTT:
		if mqtt == nil {
			return nil, fmt.Errorf("MQTT driver requested but broker is unavailable")
		}
		action := "open"
		if v, ok := cfg.Config["action"].(string); ok {
			action = v
		}
		return &MQTTDriver{client: mqtt, action: action}, nil
	case model.DriverTypeHTTP:
		return NewHTTPDriver(cfg.Config)
	case model.DriverTypeNone:
		return &NoopDriver{}, nil
	default:
		return nil, fmt.Errorf("unknown integration driver type: %q", cfg.Type)
	}
}
