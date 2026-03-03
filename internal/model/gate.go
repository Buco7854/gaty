package model

import (
	"time"

	"github.com/google/uuid"
)

const offlineThreshold = 5 * time.Minute

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

type Gate struct {
	ID                uuid.UUID           `json:"id"`
	WorkspaceID       uuid.UUID           `json:"workspace_id"`
	Name              string              `json:"name"`
	IntegrationType   GateIntegrationType `json:"integration_type"`
	IntegrationConfig map[string]any      `json:"integration_config,omitempty"`
	Status            GateStatus          `json:"status"`
	LastSeenAt        *time.Time          `json:"last_seen_at,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
}

// EffectiveStatus computes the live status based on last_seen_at:
// - never seen → unknown
// - not seen in > 5 min → offline
// - otherwise the stored status (online/offline/unknown from last MQTT message)
func (g *Gate) EffectiveStatus() GateStatus {
	if g.LastSeenAt == nil {
		return GateStatusUnknown
	}
	if time.Since(*g.LastSeenAt) > offlineThreshold {
		return GateStatusOffline
	}
	return g.Status
}
