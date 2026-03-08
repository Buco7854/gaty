// Package integration provides driver implementations for gate actions.
// Each gate can be independently configured for status, open, and close
// operations using different drivers.
package integration

import (
	"context"
	"fmt"
	"net/http"

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
// httpClient must be built via NewHTTPClient to guarantee SSRF protection.
func NewOpenDriver(gate *model.Gate, mqtt *internalmqtt.Client, httpClient *http.Client) (Driver, error) {
	if gate.OpenConfig == nil {
		return &NoopDriver{}, nil
	}
	return newDriver(gate.OpenConfig, "open", mqtt, httpClient)
}

// NewCloseDriver builds the driver for closing a gate.
// Returns a NoopDriver if close_config is nil (no action configured).
// httpClient must be built via NewHTTPClient to guarantee SSRF protection.
func NewCloseDriver(gate *model.Gate, mqtt *internalmqtt.Client, httpClient *http.Client) (Driver, error) {
	if gate.CloseConfig == nil {
		return &NoopDriver{}, nil
	}
	return newDriver(gate.CloseConfig, "close", mqtt, httpClient)
}

func newDriver(cfg *model.ActionConfig, defaultAction string, mqtt *internalmqtt.Client, httpClient *http.Client) (Driver, error) {
	switch cfg.Type {
	case model.DriverTypeMQTTGatie:
		// MQTT_GATIE: Gaty native protocol, sends {"action":"open|close"}.
		if mqtt == nil {
			return nil, fmt.Errorf("MQTT driver requested but broker is unavailable")
		}
		return &MQTTGatieDriver{client: mqtt, action: defaultAction}, nil
	case model.DriverTypeMQTTCustom:
		// MQTT_CUSTOM: publishes config["payload"] as-is.
		if mqtt == nil {
			return nil, fmt.Errorf("MQTT driver requested but broker is unavailable")
		}
		payload, _ := cfg.Config["payload"].(map[string]any)
		return &MQTTCustomDriver{client: mqtt, payload: payload}, nil
	case model.DriverTypeHTTP:
		return NewHTTPDriver(cfg.Config, httpClient)
	case model.DriverTypeNone:
		return &NoopDriver{}, nil
	default:
		return nil, fmt.Errorf("unknown integration driver type: %q", cfg.Type)
	}
}
