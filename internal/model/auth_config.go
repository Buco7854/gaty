package model

// APITokenEnabled returns true if API token authentication is allowed for a member.
// memberConfig is the per-member auth_config override (may be nil).
// wsConfig is the workspace-level member_auth_config default (may be nil).
// Per-member setting takes precedence; workspace default applies otherwise; default is enabled.
func APITokenEnabled(memberConfig, wsConfig map[string]any) bool {
	if v, ok := memberConfig["api_token"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	if v, ok := wsConfig["api_token"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return true
}
