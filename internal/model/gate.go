package model

import (
	"time"

	"github.com/google/uuid"
)

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
