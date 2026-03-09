package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

const (
	dynsecTopic     = "$CONTROL/dynamic-security/v1"
	dynsecRespTopic = "$CONTROL/dynamic-security/v1/response"
	dynsecTimeout   = 5 * time.Second
	gateRoleName    = "gate-device"
)

// DynSecManager manages MQTT client credentials via the Mosquitto
// Dynamic Security Plugin. Commands are published as JSON to the
// $CONTROL/dynamic-security/v1 topic.
//
// Implements service.BrokerAuthManager.
//
// ── Migration note ──────────────────────────────────────────────────
// This implementation is specific to Mosquitto's Dynamic Security
// Plugin. If the plugin becomes unavailable, replace DynSecManager
// with service.NoopBrokerAuth and set MQTT_AUTH_MODE=payload.
// See service.BrokerAuthManager for the full fallback procedure.
type DynSecManager struct {
	client *Client
	mu     sync.Mutex
	respCh chan json.RawMessage
}

// NewDynSecManager creates a new Dynamic Security manager.
// Call Setup() after creation to initialize roles and ACLs.
func NewDynSecManager(client *Client) *DynSecManager {
	return &DynSecManager{
		client: client,
		respCh: make(chan json.RawMessage, 1),
	}
}

// Setup subscribes to the DynSec response topic and creates the
// "gate-device" role with appropriate ACL patterns.
// Safe to call multiple times (idempotent).
func (m *DynSecManager) Setup(ctx context.Context) error {
	if err := m.client.Subscribe(dynsecRespTopic, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		select {
		case m.respCh <- json.RawMessage(msg.Payload()):
		default:
		}
	}); err != nil {
		return fmt.Errorf("dynsec: subscribe response topic: %w", err)
	}

	// Create the gate-device role with ACL patterns.
	// %u is replaced by the MQTT username (= gate UUID) at evaluation time.
	// Errors are ignored: the role may already exist from a previous startup.
	if err := m.execIgnoreExists([]map[string]any{{
		"command":  "createRole",
		"rolename": gateRoleName,
		"acls": []map[string]any{
			{"acltype": "publishClientSend", "topic": "+/gates/%u/status", "priority": -1, "allow": true},
			{"acltype": "subscribePattern", "topic": "+/gates/%u/command", "priority": -1, "allow": true},
		},
	}}); err != nil {
		slog.Warn("dynsec: failed to create gate-device role", "error", err)
	}

	// Explicitly deny all by default (defense in depth).
	_ = m.exec([]map[string]any{
		{"command": "setDefaultACLAccess", "acltype": "publishClientSend", "allow": false},
		{"command": "setDefaultACLAccess", "acltype": "subscribe", "allow": false},
	})

	slog.Info("dynsec: broker auth setup complete")
	return nil
}

// SyncCredentials creates or updates MQTT credentials for a gate.
// The gate UUID is used as the MQTT username, and the JWT token as password.
func (m *DynSecManager) SyncCredentials(_ context.Context, gateID uuid.UUID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	username := gateID.String()

	// Try createClient first; if the client already exists, update the password.
	resp, err := m.execWithResponse([]map[string]any{{
		"command":  "createClient",
		"username": username,
		"password": token,
		"roles":    []map[string]any{{"rolename": gateRoleName, "priority": -1}},
	}})
	if err != nil {
		return fmt.Errorf("dynsec: create client %s: %w", username, err)
	}

	if dynsecHasError(resp, "Client already exists") {
		if err := m.exec([]map[string]any{{
			"command":  "setClientPassword",
			"username": username,
			"password": token,
		}}); err != nil {
			return fmt.Errorf("dynsec: set password %s: %w", username, err)
		}
	} else if errMsg := dynsecFirstError(resp); errMsg != "" {
		return fmt.Errorf("dynsec: create client %s: %s", username, errMsg)
	}

	slog.Debug("dynsec: synced credentials", "gate_id", gateID)
	return nil
}

// RemoveCredentials removes MQTT credentials for a gate from the broker.
func (m *DynSecManager) RemoveCredentials(_ context.Context, gateID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ignore "not found" errors — the client may have been removed already.
	if err := m.execIgnoreExists([]map[string]any{{
		"command":  "deleteClient",
		"username": gateID.String(),
	}}); err != nil {
		slog.Warn("dynsec: failed to remove credentials", "gate_id", gateID, "error", err)
	}
	return nil
}

// exec publishes DynSec commands and waits for the response.
func (m *DynSecManager) exec(commands []map[string]any) error {
	resp, err := m.execWithResponse(commands)
	if err != nil {
		return err
	}
	if errMsg := dynsecFirstError(resp); errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// execIgnoreExists runs commands and ignores "already exists" / "not found" errors.
func (m *DynSecManager) execIgnoreExists(commands []map[string]any) error {
	resp, err := m.execWithResponse(commands)
	if err != nil {
		return err
	}
	errMsg := dynsecFirstError(resp)
	if errMsg != "" && errMsg != "Client already exists" && errMsg != "Role already exists" && errMsg != "Client not found" {
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// execWithResponse publishes commands and waits for the DynSec response.
func (m *DynSecManager) execWithResponse(commands []map[string]any) (json.RawMessage, error) {
	payload, _ := json.Marshal(map[string]any{"commands": commands})

	// Drain any stale response.
	select {
	case <-m.respCh:
	default:
	}

	if err := m.client.Publish(dynsecTopic, payload); err != nil {
		return nil, fmt.Errorf("dynsec publish: %w", err)
	}

	select {
	case resp := <-m.respCh:
		return resp, nil
	case <-time.After(dynsecTimeout):
		return nil, fmt.Errorf("dynsec: timeout waiting for response")
	}
}

// dynsecHasError checks if the DynSec response contains a specific error string.
func dynsecHasError(resp json.RawMessage, target string) bool {
	var parsed struct {
		Responses []struct {
			Error string `json:"error"`
		} `json:"responses"`
	}
	if json.Unmarshal(resp, &parsed) != nil {
		return false
	}
	for _, r := range parsed.Responses {
		if r.Error == target {
			return true
		}
	}
	return false
}

// dynsecFirstError returns the first non-empty error from a DynSec response,
// or empty string if all commands succeeded.
func dynsecFirstError(resp json.RawMessage) string {
	var parsed struct {
		Responses []struct {
			Error string `json:"error"`
		} `json:"responses"`
	}
	if json.Unmarshal(resp, &parsed) != nil {
		return ""
	}
	for _, r := range parsed.Responses {
		if r.Error != "" {
			return r.Error
		}
	}
	return ""
}
