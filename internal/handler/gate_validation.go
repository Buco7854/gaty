package handler

import (
	"fmt"
	"strings"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/safenet"
)

const (
	maxCustomStatuses    = 20
	maxCustomStatusLen   = 50
	maxStatusRules       = 50
	maxMetaFields        = 30
	maxPermissionCodeLen = 64
)

var validStatusRuleOps = map[string]bool{
	"eq": true, "ne": true, "gt": true, "gte": true, "lt": true, "lte": true,
}

// validateActionConfig validates an open or close action config.
// Valid types: MQTT_GATIE, MQTT_CUSTOM, MQTT (legacy), HTTP, NONE.
func validateActionConfig(field string, cfg *model.ActionConfig) error {
	if cfg == nil || cfg.Type == model.DriverTypeNone {
		return nil
	}
	switch cfg.Type {
	case model.DriverTypeMQTTGatie:
		// No extra config required: topic is derived from workspace_id + gate_id.
	case model.DriverTypeMQTTCustom:
		payload, _ := cfg.Config["payload"].(map[string]any)
		if payload == nil {
			return fmt.Errorf("%s: MQTT_CUSTOM driver requires a non-empty 'payload' object in config", field)
		}
	case model.DriverTypeHTTP:
		url, _ := cfg.Config["url"].(string)
		if strings.TrimSpace(url) == "" {
			return fmt.Errorf("%s: HTTP driver requires a non-empty url", field)
		}
		if err := safenet.ValidateURL(url); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if m, ok := cfg.Config["method"].(string); ok && m != "" {
			if err := safenet.ValidateHTTPMethod(m); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
	default:
		return fmt.Errorf("%s: driver type %q is not valid for open/close actions (allowed: MQTT_GATIE, MQTT_CUSTOM, HTTP, NONE)", field, cfg.Type)
	}
	return nil
}

// validateStatusActionConfig validates a status source configuration.
// Only MQTT, HTTP_INBOUND, HTTP_WEBHOOK, and NONE are valid for status_config.
// - HTTP_WEBHOOK requires a non-empty url.
// - All active modes require a payload mapping with a non-empty status field.
// - Status value mappings are always required and must cover both "open" and "closed".
func validateStatusActionConfig(cfg *model.ActionConfig) error {
	if cfg == nil || cfg.Type == model.DriverTypeNone {
		return nil
	}

	switch cfg.Type {
	case model.DriverTypeMQTTGatie:
		// MQTT_GATIE: no mapping needed, format is fixed by the protocol.
		return nil
	case model.DriverTypeMQTTCustom, model.DriverTypeHTTPInbound, model.DriverTypeHTTPWebhook:
		// valid types requiring mapping validation below
	default:
		return fmt.Errorf("status_config: driver type %q is not valid for status (allowed: MQTT_GATIE, MQTT_CUSTOM, HTTP_INBOUND, HTTP_WEBHOOK, NONE)", cfg.Type)
	}

	if cfg.Type == model.DriverTypeHTTPWebhook {
		url, _ := cfg.Config["url"].(string)
		if strings.TrimSpace(url) == "" {
			return fmt.Errorf("status_config: HTTP_WEBHOOK driver requires a non-empty url")
		}
		if err := safenet.ValidateURL(url); err != nil {
			return fmt.Errorf("status_config: %w", err)
		}
		if m, ok := cfg.Config["method"].(string); ok && m != "" {
			if err := safenet.ValidateHTTPMethod(m); err != nil {
				return fmt.Errorf("status_config: %w", err)
			}
		}
		if v, ok := cfg.Config["interval_seconds"]; ok {
			var interval float64
			switch n := v.(type) {
			case float64:
				interval = n
			case int:
				interval = float64(n)
			}
			if interval < 10 || interval > 3600 {
				return fmt.Errorf("status_config: interval_seconds must be between 10 and 3600")
			}
		}
	}

	// All active modes require a complete payload mapping (field mandatory).
	mapping, ok := model.ExtractPayloadMapping(cfg.Config)
	if !ok {
		return fmt.Errorf("status_config: a payload mapping with a non-empty status field is required for %s mode", cfg.Type)
	}

	// Value mappings are always required: "open" and "closed" must be reachable.
	if len(mapping.Status.Values) == 0 {
		return fmt.Errorf("status_config: status value mapping must define at least 'open' and 'closed' targets")
	}
	mapped := make(map[string]bool, len(mapping.Status.Values))
	for _, v := range mapping.Status.Values {
		mapped[v] = true
	}
	var missing []string
	for _, req := range []string{"open", "closed"} {
		if !mapped[req] {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("status_config: status value mapping is missing required targets: %s", strings.Join(missing, ", "))
	}
	return nil
}

// validateCustomStatuses checks that user-defined statuses do not collide with
// built-in statuses and contain no duplicates or empty entries.
func validateCustomStatuses(statuses []string) error {
	if len(statuses) > maxCustomStatuses {
		return fmt.Errorf("custom_statuses: cannot define more than %d custom statuses", maxCustomStatuses)
	}
	defaultSet := make(map[string]bool, len(model.DefaultGateStatuses))
	for _, s := range model.DefaultGateStatuses {
		defaultSet[s] = true
	}
	seen := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("custom_statuses: status name must not be empty")
		}
		if len(s) > maxCustomStatusLen {
			return fmt.Errorf("custom_statuses: status name must not exceed %d characters", maxCustomStatusLen)
		}
		if defaultSet[s] {
			return fmt.Errorf("custom_statuses: %q is a built-in status and cannot be redefined", s)
		}
		if seen[s] {
			return fmt.Errorf("custom_statuses: duplicate status %q", s)
		}
		seen[s] = true
	}
	return nil
}

// validateStatusRules checks that each rule has a valid operator, a non-empty key,
// and a set_status that resolves to a known status (defaults + custom).
func validateStatusRules(rules []model.StatusRule, customStatuses []string) error {
	if len(rules) > maxStatusRules {
		return fmt.Errorf("status_rules: cannot define more than %d rules", maxStatusRules)
	}
	known := make(map[string]bool, len(model.DefaultGateStatuses)+len(customStatuses))
	for _, s := range model.DefaultGateStatuses {
		known[s] = true
	}
	for _, s := range customStatuses {
		known[s] = true
	}
	for i, rule := range rules {
		if strings.TrimSpace(rule.Key) == "" {
			return fmt.Errorf("status_rules[%d]: key must not be empty", i)
		}
		if !validStatusRuleOps[rule.Op] {
			return fmt.Errorf("status_rules[%d]: invalid operator %q (allowed: eq, ne, gt, gte, lt, lte)", i, rule.Op)
		}
		if strings.TrimSpace(rule.SetStatus) == "" {
			return fmt.Errorf("status_rules[%d]: set_status must not be empty", i)
		}
		if !known[rule.SetStatus] {
			return fmt.Errorf("status_rules[%d]: set_status %q is not a known status", i, rule.SetStatus)
		}
	}
	return nil
}

// ValidPermissionCodes is the set of permission codes defined in the database.
var ValidPermissionCodes = map[string]bool{
	"gate:read_status":   true,
	"gate:trigger_open":  true,
	"gate:trigger_close": true,
	"gate:manage":        true,
}

// validatePermissionCode checks that a permission code is known and within length limits.
func validatePermissionCode(code string) error {
	if len(code) > maxPermissionCodeLen {
		return fmt.Errorf("permission_code must not exceed %d characters", maxPermissionCodeLen)
	}
	if !ValidPermissionCodes[code] {
		return fmt.Errorf("permission_code %q is not a valid permission", code)
	}
	return nil
}

// validateMetaConfig checks bounds on meta_config entries.
func validateMetaConfig(fields []model.MetaField) error {
	if len(fields) > maxMetaFields {
		return fmt.Errorf("meta_config: cannot define more than %d fields", maxMetaFields)
	}
	return nil
}
