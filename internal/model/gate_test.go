package model

import (
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────────
// EvaluateStatusRules
// ────────────────────────────────────────────────────────────────────

func TestEvaluateStatusRules_NoRules(t *testing.T) {
	status, matched := EvaluateStatusRules(nil, map[string]any{"battery": 50.0})
	if matched {
		t.Error("expected no match with empty rules")
	}
	if status != "" {
		t.Errorf("expected empty status, got %q", status)
	}
}

func TestEvaluateStatusRules_FirstMatchWins(t *testing.T) {
	rules := []StatusRule{
		{Key: "battery", Op: "lt", Value: "20", SetStatus: "low_battery"},
		{Key: "battery", Op: "lt", Value: "50", SetStatus: "medium_battery"},
	}
	meta := map[string]any{"battery": 10.0}

	status, matched := EvaluateStatusRules(rules, meta)
	if !matched {
		t.Fatal("expected a match")
	}
	if status != "low_battery" {
		t.Errorf("expected low_battery, got %q", status)
	}
}

func TestEvaluateStatusRules_NoMatch(t *testing.T) {
	rules := []StatusRule{
		{Key: "battery", Op: "lt", Value: "20", SetStatus: "low_battery"},
	}
	meta := map[string]any{"battery": 80.0}

	_, matched := EvaluateStatusRules(rules, meta)
	if matched {
		t.Error("expected no match")
	}
}

func TestEvaluateStatusRules_MissingKey(t *testing.T) {
	rules := []StatusRule{
		{Key: "nonexistent", Op: "eq", Value: "foo", SetStatus: "custom"},
	}
	_, matched := EvaluateStatusRules(rules, map[string]any{"battery": 50.0})
	if matched {
		t.Error("expected no match for missing key")
	}
}

// ────────────────────────────────────────────────────────────────────
// ruleMatches — numeric operators
// ────────────────────────────────────────────────────────────────────

func TestRuleMatches_Numeric(t *testing.T) {
	cases := []struct {
		op        string
		actual    any
		threshold string
		want      bool
	}{
		{"eq", 42.0, "42", true},
		{"eq", 42.0, "43", false},
		{"ne", 42.0, "43", true},
		{"ne", 42.0, "42", false},
		{"gt", 10.0, "5", true},
		{"gt", 5.0, "10", false},
		{"gte", 10.0, "10", true},
		{"gte", 9.0, "10", false},
		{"lt", 5.0, "10", true},
		{"lt", 10.0, "5", false},
		{"lte", 10.0, "10", true},
		{"lte", 11.0, "10", false},
		// int types
		{"gt", int(10), "5", true},
		{"lt", int64(3), "5", true},
		// string numeric
		{"gt", "15", "10", true},
	}

	for _, tc := range cases {
		got := ruleMatches(tc.op, tc.actual, tc.threshold)
		if got != tc.want {
			t.Errorf("ruleMatches(%q, %v, %q) = %v, want %v", tc.op, tc.actual, tc.threshold, got, tc.want)
		}
	}
}

func TestRuleMatches_StringEquality(t *testing.T) {
	if !ruleMatches("eq", "open", "open") {
		t.Error("expected string eq match")
	}
	if ruleMatches("eq", "open", "closed") {
		t.Error("expected no string eq match")
	}
	if !ruleMatches("ne", "open", "closed") {
		t.Error("expected string ne match")
	}
}

func TestRuleMatches_InvalidThreshold(t *testing.T) {
	// numeric actual but non-numeric threshold → no match
	if ruleMatches("gt", 10.0, "notanumber") {
		t.Error("expected no match with invalid threshold")
	}
}

func TestRuleMatches_UnknownOp(t *testing.T) {
	if ruleMatches("bogus", 10.0, "5") {
		t.Error("expected no match for unknown operator")
	}
}

// ────────────────────────────────────────────────────────────────────
// getNestedValue — dot notation
// ────────────────────────────────────────────────────────────────────

func TestGetNestedValue_FlatKey(t *testing.T) {
	m := map[string]any{"battery": 80.0}
	v, ok := getNestedValue(m, "battery")
	if !ok || v != 80.0 {
		t.Errorf("expected 80.0, got %v ok=%v", v, ok)
	}
}

func TestGetNestedValue_DotNotation(t *testing.T) {
	m := map[string]any{
		"lora": map[string]any{"snr": -8.0},
	}
	v, ok := getNestedValue(m, "lora.snr")
	if !ok || v != -8.0 {
		t.Errorf("expected -8.0, got %v ok=%v", v, ok)
	}
}

func TestGetNestedValue_FlatKeyWithDotTakesPriority(t *testing.T) {
	// A flat key "a.b" should take priority over nested resolution.
	m := map[string]any{
		"a.b": "flat",
		"a":   map[string]any{"b": "nested"},
	}
	v, ok := getNestedValue(m, "a.b")
	if !ok || v != "flat" {
		t.Errorf("expected flat key to win, got %v ok=%v", v, ok)
	}
}

func TestGetNestedValue_MissingKey(t *testing.T) {
	_, ok := getNestedValue(map[string]any{}, "missing")
	if ok {
		t.Error("expected missing key to return ok=false")
	}
}

func TestGetNestedValue_MissingNestedKey(t *testing.T) {
	m := map[string]any{"a": map[string]any{"x": 1}}
	_, ok := getNestedValue(m, "a.b")
	if ok {
		t.Error("expected missing nested key to return ok=false")
	}
}

func TestGetNestedValue_IntermediateNotAMap(t *testing.T) {
	m := map[string]any{"a": "not-a-map"}
	_, ok := getNestedValue(m, "a.b")
	if ok {
		t.Error("expected false when intermediate value is not a map")
	}
}

// ────────────────────────────────────────────────────────────────────
// EffectiveStatus
// ────────────────────────────────────────────────────────────────────

func TestEffectiveStatus_NeverSeen(t *testing.T) {
	g := &Gate{Status: GateStatusOnline}
	if g.EffectiveStatus() != GateStatusUnknown {
		t.Error("expected unknown when never seen")
	}
}

func TestEffectiveStatus_RecentlySeen(t *testing.T) {
	now := time.Now()
	g := &Gate{Status: GateStatusOnline, LastSeenAt: &now}
	if g.EffectiveStatus() != GateStatusOnline {
		t.Error("expected online for recently seen gate")
	}
}

func TestEffectiveStatus_OldSeen(t *testing.T) {
	old := time.Now().Add(-OfflineThreshold - time.Second)
	g := &Gate{Status: GateStatusOnline, LastSeenAt: &old}
	if g.EffectiveStatus() != GateStatusOffline {
		t.Errorf("expected offline for gate last seen >%v ago, got %q", OfflineThreshold, g.EffectiveStatus())
	}
}

func TestEffectiveStatus_ExactlyOnThreshold(t *testing.T) {
	// At exactly the threshold boundary (not past it), still show stored status.
	threshold := time.Now().Add(-OfflineThreshold + time.Millisecond)
	g := &Gate{Status: GateStatusUnresponsive, LastSeenAt: &threshold}
	// Should return stored status since threshold not crossed
	if g.EffectiveStatus() != GateStatusUnresponsive {
		t.Errorf("expected unresponsive, got %q", g.EffectiveStatus())
	}
}

// ────────────────────────────────────────────────────────────────────
// DefaultGateStatuses
// ────────────────────────────────────────────────────────────────────

func TestDefaultGateStatuses(t *testing.T) {
	expected := []string{"open", "closed", "unavailable"}
	if len(DefaultGateStatuses) != len(expected) {
		t.Fatalf("expected %d default statuses, got %d", len(expected), len(DefaultGateStatuses))
	}
	for i, s := range expected {
		if DefaultGateStatuses[i] != s {
			t.Errorf("DefaultGateStatuses[%d] = %q, want %q", i, DefaultGateStatuses[i], s)
		}
	}
}
