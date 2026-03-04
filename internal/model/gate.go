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
