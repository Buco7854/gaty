package model

import (
	"time"

	"github.com/google/uuid"
)

const offlineThreshold = 5 * time.Minute

// GateIntegrationType is kept for backward compatibility with existing records.
type GateIntegrationType string

const (
	IntegrationTypeMQTT    GateIntegrationType = "MQTT"
	IntegrationTypePolling GateIntegrationType = "POLLING"
	IntegrationTypeWebhook GateIntegrationType = "WEBHOOK"
)

type GateStatus string

const (
	GateStatusOnline  GateStatus = "online"
	GateStatusOffline GateStatus = "offline"
	GateStatusUnknown GateStatus = "unknown"
)

// DriverType identifies which integration driver handles a gate action.
type DriverType string

const (
	DriverTypeMQTT DriverType = "MQTT"
	DriverTypeHTTP DriverType = "HTTP"
	DriverTypeNone DriverType = "NONE"
)

// ActionConfig describes how to execute a specific gate action (open, close, status).
// Each gate action can independently use a different driver and configuration.
type ActionConfig struct {
	Type   DriverType     `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// MetaField describes how a raw status-payload key maps to a display label.
// For example: Key="lora.snr", Label="SNR", Unit="dB".
// The frontend shows this field when the user has gate:read_status permission.
type MetaField struct {
	// Key is the dot-notated path in the status payload's "meta" object.
	Key string `json:"key" minLength:"1"`
	// Label is the human-readable name shown in the UI.
	Label string `json:"label" minLength:"1"`
	// Unit is an optional suffix displayed next to the value (e.g. "dB", "%").
	Unit string `json:"unit,omitempty"`
}

type Gate struct {
	ID                uuid.UUID           `json:"id"`
	WorkspaceID       uuid.UUID           `json:"workspace_id"`
	Name              string              `json:"name"`
	IntegrationType   GateIntegrationType `json:"integration_type"` // kept for backward compat
	IntegrationConfig map[string]any      `json:"integration_config,omitempty"`

	// Per-action integration configs (preferred over legacy fields).
	// nil means "use legacy fallback" or "not configured".
	OpenConfig   *ActionConfig `json:"open_config,omitempty"`
	CloseConfig  *ActionConfig `json:"close_config,omitempty"`
	StatusConfig *ActionConfig `json:"status_config,omitempty"`

	Status     GateStatus `json:"status"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`

	// StatusMetadata holds the last payload's "meta" object received from the gate.
	// Shown to users with gate:read_status permission.
	StatusMetadata map[string]any `json:"status_metadata,omitempty"`

	// MetaConfig configures which metadata fields to display and how to label them.
	MetaConfig []MetaField `json:"meta_config,omitempty"`

	// GateToken is the gate's authentication secret.
	// Only populated in create and rotate-token responses (never in list/get).
	GateToken *string `json:"gate_token,omitempty"`
}

// EffectiveStatus computes the live status based on last_seen_at:
//   - never seen → unknown
//   - not seen in > 5 min → offline
//   - otherwise the stored status (from last MQTT message or poll)
func (g *Gate) EffectiveStatus() GateStatus {
	if g.LastSeenAt == nil {
		return GateStatusUnknown
	}
	if time.Since(*g.LastSeenAt) > offlineThreshold {
		return GateStatusOffline
	}
	return g.Status
}
