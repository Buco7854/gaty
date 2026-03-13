package handler

import (
	"fmt"
	"strings"

	"github.com/Buco7854/gatie/internal/model"
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
		// No extra config required: topic is derived from gate_id.
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
		if err := validateSuccessStatusCodes(cfg.Config); err != nil {
			return err
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

// validateTTLSeconds checks that a per-gate TTL override is within a sensible range.
func validateTTLSeconds(v *int) error {
	if v == nil {
		return nil
	}
	if *v < 1 || *v > 3600 {
		return fmt.Errorf("ttl_seconds must be between 1 and 3600")
	}
	return nil
}

// validateStatusTransitions checks that each transition has valid from/to statuses,
// from != to, and after_seconds > 0.
func validateStatusTransitions(transitions []model.StatusTransition, customStatuses []string) error {
	known := make(map[string]bool, len(model.DefaultGateStatuses)+len(customStatuses))
	for _, s := range model.DefaultGateStatuses {
		known[s] = true
	}
	for _, s := range customStatuses {
		known[s] = true
	}
	for i, t := range transitions {
		if t.From == t.To {
			return fmt.Errorf("status_transitions[%d]: 'from' and 'to' must be different", i)
		}
		if t.AfterSeconds <= 0 {
			return fmt.Errorf("status_transitions[%d]: after_seconds must be > 0", i)
		}
		if !known[t.From] {
			return fmt.Errorf("status_transitions[%d]: 'from' status %q is not a known status", i, t.From)
		}
		if !known[t.To] {
			return fmt.Errorf("status_transitions[%d]: 'to' status %q is not a known status", i, t.To)
		}
	}
	return nil
}

// validateSuccessStatusCodes validates the optional success_status_codes field in a webhook config.
func validateSuccessStatusCodes(cfg map[string]any) error {
	raw, ok := cfg["success_status_codes"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("status_config: success_status_codes must be an array of {from, to} ranges")
	}
	for i, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("status_config: success_status_codes[%d] must be an object with 'from' and 'to'", i)
		}
		from, okF := model.ToInt(m["from"])
		to, okT := model.ToInt(m["to"])
		if !okF || !okT {
			return fmt.Errorf("status_config: success_status_codes[%d]: 'from' and 'to' must be integers", i)
		}
		if from > to {
			return fmt.Errorf("status_config: success_status_codes[%d]: 'from' (%d) must be <= 'to' (%d)", i, from, to)
		}
		if from < 100 || to > 599 {
			return fmt.Errorf("status_config: success_status_codes[%d]: values must be between 100 and 599", i)
		}
	}
	return nil
}

// validateCustomStatuses checks that user-defined statuses do not collide with
// built-in statuses and contain no duplicates or empty entries.
func validateCustomStatuses(statuses []string) error {
	defaultSet := make(map[string]bool, len(model.DefaultGateStatuses))
	for _, s := range model.DefaultGateStatuses {
		defaultSet[s] = true
	}
	seen := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("custom_statuses: status name must not be empty")
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
