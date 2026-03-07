package model

import (
	"encoding/json"
	"fmt"
)

// PayloadFormat identifies the serialization format of a device payload.
// Only "json" is supported currently; the type exists for future expansion (e.g. "xml").
type PayloadFormat string

const PayloadFormatJSON PayloadFormat = "json"

// StatusFieldMapping defines which field in the incoming payload carries the gate status
// and how its raw values map to application status strings.
//
// Example: field="state", values={"1":"open","0":"closed"}
// → payload {"state":1} → status "open"
type StatusFieldMapping struct {
	// Field is the dot-notated path to the status value (e.g. "state", "data.status").
	Field string `json:"field" minLength:"1"`
	// Values maps raw device values (converted to string) to app status strings.
	// If empty, the raw string value is used as-is.
	Values map[string]string `json:"values,omitempty"`
}

// PayloadMapping configures how to parse an incoming status payload.
// Stored in StatusConfig.Config["mapping"] for MQTT_CUSTOM, HTTP_INBOUND, and HTTP_WEBHOOK modes.
// Metadata extraction is handled separately via gate.MetaConfig for all modes.
type PayloadMapping struct {
	// Format is the payload serialization format. Defaults to "json".
	// Only "json" is currently supported; the field exists for future formats (e.g. "xml").
	Format PayloadFormat `json:"format,omitempty"`
	// Status defines how to locate and interpret the gate status in the payload.
	Status StatusFieldMapping `json:"status"`
}

// ExtractPayloadMapping parses a PayloadMapping from a StatusConfig.Config map.
// Returns (nil, false) if the "mapping" key is absent, nil, or malformed.
func ExtractPayloadMapping(config map[string]any) (*PayloadMapping, bool) {
	raw, ok := config["mapping"]
	if !ok || raw == nil {
		return nil, false
	}
	// Re-marshal the map[string]any to JSON then unmarshal into the typed struct.
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var m PayloadMapping
	if json.Unmarshal(b, &m) != nil || m.Status.Field == "" {
		return nil, false
	}
	return &m, true
}

// ApplyMapping extracts the gate status from a raw flat/nested payload map
// using the provided PayloadMapping. Only "json" format is currently supported.
//
// The status field is located via dot notation. Its raw value is converted to a string
// and looked up in StatusFieldMapping.Values; if Values is empty, the raw string is used.
//
// Returns an error if the status field is missing, the value has no mapping, or the
// format is unsupported.
//
// Metadata extraction is handled separately via ExtractMeta using gate.MetaConfig.
func ApplyMapping(mapping PayloadMapping, raw map[string]any) (status string, err error) {
	if mapping.Format != "" && mapping.Format != PayloadFormatJSON {
		return "", fmt.Errorf("unsupported payload format: %q", mapping.Format)
	}

	rawVal, ok := getNestedValue(raw, mapping.Status.Field)
	if !ok {
		return "", fmt.Errorf("status field %q not found in payload", mapping.Status.Field)
	}
	rawStr := fmt.Sprintf("%v", rawVal)

	if len(mapping.Status.Values) > 0 {
		mapped, found := mapping.Status.Values[rawStr]
		if !found {
			return "", fmt.Errorf("status value %q has no mapping", rawStr)
		}
		return mapped, nil
	}
	return rawStr, nil
}

// ExtractMeta reads the fields listed in metaConfig from raw using dot-notation keys.
// Only configured fields are extracted — no extra payload fields are stored.
func ExtractMeta(metaConfig []MetaField, raw map[string]any) map[string]any {
	if len(metaConfig) == 0 {
		return nil
	}
	meta := make(map[string]any, len(metaConfig))
	for _, field := range metaConfig {
		if v, ok := getNestedValue(raw, field.Key); ok {
			meta[field.Key] = v
		}
	}
	return meta
}
