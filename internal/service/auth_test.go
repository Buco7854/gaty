package service

import (
	"testing"
)

// ────────────────────────────────────────────────────────────────────
// validatePassword
// ────────────────────────────────────────────────────────────────────

func newTestAuthService(policy PasswordPolicy) *AuthService {
	return &AuthService{passwordPolicy: policy}
}

func TestValidatePassword_TooShort(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 8})
	if err := svc.validatePassword("Abc1"); err == nil {
		t.Error("expected error for password too short")
	}
}

func TestValidatePassword_ExactMinLength(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 8, RequireUpper: true, RequireLower: true, RequireDigit: true})
	if err := svc.validatePassword("Abcde1fg"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePassword_MissingUpper(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 4, RequireUpper: true})
	if err := svc.validatePassword("abcd1234"); err == nil {
		t.Error("expected error for missing uppercase")
	}
}

func TestValidatePassword_MissingLower(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 4, RequireLower: true})
	if err := svc.validatePassword("ABCD1234"); err == nil {
		t.Error("expected error for missing lowercase")
	}
}

func TestValidatePassword_MissingDigit(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 4, RequireDigit: true})
	if err := svc.validatePassword("Abcdefgh"); err == nil {
		t.Error("expected error for missing digit")
	}
}

func TestValidatePassword_AllRequirementsDisabled(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 4})
	// Any 4+ char string is valid when no complexity is required
	if err := svc.validatePassword("aaaa"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePassword_MinLengthZero(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{})
	// Zero min length: empty string is valid
	if err := svc.validatePassword(""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePassword_UnicodeCharacters(t *testing.T) {
	svc := newTestAuthService(PasswordPolicy{MinLength: 4, RequireUpper: true, RequireLower: true, RequireDigit: true})
	// Unicode uppercase and lowercase
	if err := svc.validatePassword("Héllo1!"); err != nil {
		t.Errorf("unexpected error with unicode: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────
// refreshKey — deterministic SHA-256 hash
// ────────────────────────────────────────────────────────────────────

func TestRefreshKey_Deterministic(t *testing.T) {
	k1 := refreshKey("my-token")
	k2 := refreshKey("my-token")
	if k1 != k2 {
		t.Error("refreshKey must be deterministic for the same input")
	}
}

func TestRefreshKey_DifferentInputs(t *testing.T) {
	k1 := refreshKey("token-a")
	k2 := refreshKey("token-b")
	if k1 == k2 {
		t.Error("different inputs must produce different keys")
	}
}

func TestRefreshKey_HasPrefix(t *testing.T) {
	k := refreshKey("any-token")
	if len(k) <= len(refreshKeyPrefix) || k[:len(refreshKeyPrefix)] != refreshKeyPrefix {
		t.Errorf("refreshKey should start with %q, got %q", refreshKeyPrefix, k)
	}
}

func TestRefreshKey_NeverStoresRawToken(t *testing.T) {
	token := "supersecrettoken"
	k := refreshKey(token)
	if k == refreshKeyPrefix+token {
		t.Error("refreshKey must not embed the raw token")
	}
}

// ────────────────────────────────────────────────────────────────────
// resolveSessionDuration
// ────────────────────────────────────────────────────────────────────

func TestResolveSessionDuration_Nil(t *testing.T) {
	d := resolveSessionDuration(nil)
	if d != defaultSessionTTL {
		t.Errorf("nil config should return defaultSessionTTL, got %v", d)
	}
}

func TestResolveSessionDuration_MissingKey(t *testing.T) {
	d := resolveSessionDuration(map[string]any{"other_key": "value"})
	if d != defaultSessionTTL {
		t.Errorf("missing session_duration key should return defaultSessionTTL, got %v", d)
	}
}

func TestResolveSessionDuration_Zero(t *testing.T) {
	d := resolveSessionDuration(map[string]any{"session_duration": float64(0)})
	if d != 0 {
		t.Errorf("session_duration=0 should return 0 (infinite), got %v", d)
	}
}

func TestResolveSessionDuration_Custom(t *testing.T) {
	d := resolveSessionDuration(map[string]any{"session_duration": float64(3600)})
	if d.Seconds() != 3600 {
		t.Errorf("expected 3600s, got %v", d)
	}
}

func TestResolveSessionDuration_Negative(t *testing.T) {
	// Negative values fall back to default
	d := resolveSessionDuration(map[string]any{"session_duration": float64(-100)})
	if d != defaultSessionTTL {
		t.Errorf("negative session_duration should return defaultSessionTTL, got %v", d)
	}
}

// ────────────────────────────────────────────────────────────────────
// payloadSessionDuration
// ────────────────────────────────────────────────────────────────────

func TestPayloadSessionDuration_Missing(t *testing.T) {
	d := payloadSessionDuration(map[string]any{})
	if d != defaultSessionTTL {
		t.Errorf("expected defaultSessionTTL, got %v", d)
	}
}

func TestPayloadSessionDuration_Custom(t *testing.T) {
	d := payloadSessionDuration(map[string]any{"session_duration": float64(7200)})
	if d.Seconds() != 7200 {
		t.Errorf("expected 7200s, got %v", d)
	}
}

func TestPayloadSessionDuration_Zero(t *testing.T) {
	d := payloadSessionDuration(map[string]any{"session_duration": float64(0)})
	if d != 0 {
		t.Errorf("expected 0 (infinite), got %v", d)
	}
}
