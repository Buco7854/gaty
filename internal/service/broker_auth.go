package service

import (
	"context"

	"github.com/google/uuid"
)

// BrokerAuthManager manages MQTT client credentials at the broker level.
// Implementations sync gate tokens with the MQTT broker so that gates
// authenticate at CONNECT time (username=gate_id, password=token).
//
// ── Switching auth strategies ───────────────────────────────────────
// To fall back from broker-level auth (Mosquitto Dynamic Security) to
// app-level auth (token validated in each MQTT payload):
//
//  1. Set MQTT_AUTH_MODE=payload
//  2. Use NoopBrokerAuth here (automatic via config)
//  3. SubscribeGateStatuses in mqtt/gate.go will validate tokens
//     from the payload (brokerAuth=false branch). No other code
//     changes are needed.
//
// This design ensures a seamless migration if the Dynamic Security
// Plugin ever becomes unavailable or if a different broker is adopted.
type BrokerAuthManager interface {
	// SyncCredentials creates or updates MQTT credentials for a gate.
	// Called on gate creation and token rotation.
	SyncCredentials(ctx context.Context, gateID uuid.UUID, token string) error

	// RemoveCredentials removes MQTT credentials for a gate.
	// Called on gate deletion.
	RemoveCredentials(ctx context.Context, gateID uuid.UUID) error
}

// NoopBrokerAuth is a no-op implementation used when MQTT authentication
// happens at the application level (MQTT_AUTH_MODE=payload).
// Gate tokens are validated in each MQTT payload by SubscribeGateStatuses.
type NoopBrokerAuth struct{}

func (NoopBrokerAuth) SyncCredentials(context.Context, uuid.UUID, string) error { return nil }
func (NoopBrokerAuth) RemoveCredentials(context.Context, uuid.UUID) error       { return nil }
