// Package integration provides driver implementations for gate actions.
// Each gate can be independently configured for status, open, and close
// operations using different drivers.
package integration

import (
	"context"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	internalmqtt "github.com/Buco7854/gatie/internal/mqtt"
)

// Driver can execute an action on a gate.
type Driver interface {
	// Execute sends the action payload to the gate.
	Execute(ctx context.Context, gate *model.Gate) error
}

// NewOpenDriver builds the driver to use for opening a gate.
// Returns a NoopDriver if open_config is nil (no action configured).
func NewOpenDriver(gate *model.Gate, mqtt *internalmqtt.Client) (Driver, error) {
	if gate.OpenConfig == nil {
		return &NoopDriver{}, nil
	}
	return newDriver(gate.OpenConfig, "open", mqtt)
}

// NewCloseDriver builds the driver for closing a gate.
// Returns a NoopDriver if close_config is nil (no action configured).
func NewCloseDriver(gate *model.Gate, mqtt *internalmqtt.Client) (Driver, error) {
	if gate.CloseConfig == nil {
		return &NoopDriver{}, nil
	}
	return newDriver(gate.CloseConfig, "close", mqtt)
}

func newDriver(cfg *model.ActionConfig, defaultAction string, mqtt *internalmqtt.Client) (Driver, error) {
	switch cfg.Type {
	case model.DriverTypeMQTT:
		if mqtt == nil {
			return nil, fmt.Errorf("MQTT driver requested but broker is unavailable")
		}
		action := defaultAction
		if v, ok := cfg.Config["action"].(string); ok && v != "" {
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
