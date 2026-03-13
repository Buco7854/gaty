package model

// APITokenEnabled returns true if API token authentication is allowed for a member.
// Default is enabled.
func APITokenEnabled(memberConfig map[string]any) bool {
	if v, ok := memberConfig["api_token"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return true
}
