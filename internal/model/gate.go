package model

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// OfflineThreshold is the duration after which a gate that has stopped
// reporting is shown as "offline" in API responses (computed by EffectiveStatus).
//
// The TTL worker (service.DefaultGateTTL = 30s) first marks the gate
// "unresponsive" in the database. Once past OfflineThreshold, the displayed
// status transitions to "offline" regardless of the stored value — indicating
// a longer-term connectivity loss rather than a momentary hiccup.
const OfflineThreshold = 5 * time.Minute

// GateIntegrationType is kept for backward compatibility with existing records.
type GateIntegrationType string

const (
	IntegrationTypeMQTT    GateIntegrationType = "MQTT"
	IntegrationTypePolling GateIntegrationType = "POLLING"
	IntegrationTypeWebhook GateIntegrationType = "WEBHOOK"
)

type GateStatus string

const (
	GateStatusOnline       GateStatus = "online"
	GateStatusOffline      GateStatus = "offline"
	GateStatusUnknown      GateStatus = "unknown"
	GateStatusUnresponsive GateStatus = "unresponsive"
)

// StatusRule defines a condition evaluated against status metadata.
// When the condition matches, the gate status is overridden with SetStatus.
//
// Example: {Key: "battery", Op: "lt", Value: "20", SetStatus: "low_battery"}
// → if meta["battery"] < 20 → status becomes "low_battery"
type StatusRule struct {
	// Key is the dot-notated path in the status payload's "meta" object.
	Key string `json:"key" minLength:"1"`
	// Op is the comparison operator: "eq", "ne", "gt", "gte", "lt", "lte"
	Op string `json:"op" minLength:"2"`
	// Value is the threshold as a string. Numeric comparisons convert both sides to float64.
	Value string `json:"value"`
	// SetStatus is the gate status to set when this rule matches.
	SetStatus string `json:"set_status" minLength:"1"`
}

// EvaluateStatusRules checks each rule against meta in order and returns the first matching
// (setStatus, true), or ("", false) if no rule matches.
func EvaluateStatusRules(rules []StatusRule, meta map[string]any) (string, bool) {
	for _, rule := range rules {
		val, ok := meta[rule.Key]
		if !ok {
			continue
		}
		if ruleMatches(rule.Op, val, rule.Value) {
			return rule.SetStatus, true
		}
	}
	return "", false
}

func ruleMatches(op string, actual any, threshold string) bool {
	// Try numeric comparison first
	var actualF float64
	switch v := actual.(type) {
	case float64:
		actualF = v
	case float32:
		actualF = float64(v)
	case int:
		actualF = float64(v)
	case int64:
		actualF = float64(v)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			actualF = f
		} else {
			// Fall through to string equality for eq/ne
			switch op {
			case "eq":
				return v == threshold
			case "ne":
				return v != threshold
			}
			return false
		}
	default:
		// Compare via string representation for eq/ne
		s := fmt.Sprintf("%v", actual)
		switch op {
		case "eq":
			return s == threshold
		case "ne":
			return s != threshold
		}
		return false
	}

	threshF, err := strconv.ParseFloat(threshold, 64)
	if err != nil {
		return false
	}
	switch op {
	case "eq":
		return actualF == threshF
	case "ne":
		return actualF != threshF
	case "gt":
		return actualF > threshF
	case "gte":
		return actualF >= threshF
	case "lt":
		return actualF < threshF
	case "lte":
		return actualF <= threshF
	}
	return false
}

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

	// StatusRules are evaluated against incoming metadata to override the reported status.
	// Rules are evaluated in order; the first match wins.
	StatusRules []StatusRule `json:"status_rules,omitempty"`

	// GateToken is the gate's authentication secret.
	// Only populated in create and rotate-token responses (never in list/get).
	GateToken *string `json:"gate_token,omitempty"`
}

// EffectiveStatus computes the live status based on last_seen_at:
//   - never seen → unknown
//   - not seen in > OfflineThreshold (5 min) → offline
//   - otherwise the stored status (from last MQTT message or poll)
func (g *Gate) EffectiveStatus() GateStatus {
	if g.LastSeenAt == nil {
		return GateStatusUnknown
	}
	if time.Since(*g.LastSeenAt) > OfflineThreshold {
		return GateStatusOffline
	}
	return g.Status
}
