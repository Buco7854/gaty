package service

import (
	"testing"

	"github.com/google/uuid"
)

var testSecret = []byte("test-secret-key-32-bytes-long-ok!")

func TestIssueAndParseGateToken_RoundTrip(t *testing.T) {
	gateID := uuid.New()

	token, err := IssueGateToken(gateID, testSecret)
	if err != nil {
		t.Fatalf("IssueGateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	parsedGateID, err := ParseGateToken(token, testSecret)
	if err != nil {
		t.Fatalf("ParseGateToken failed: %v", err)
	}
	if parsedGateID != gateID {
		t.Errorf("gateID mismatch: got %v, want %v", parsedGateID, gateID)
	}
}

func TestParseGateToken_WrongSecret(t *testing.T) {
	gateID := uuid.New()

	token, err := IssueGateToken(gateID, testSecret)
	if err != nil {
		t.Fatalf("IssueGateToken failed: %v", err)
	}

	_, err = ParseGateToken(token, []byte("wrong-secret"))
	if err == nil {
		t.Error("expected error when parsing with wrong secret")
	}
}

func TestParseGateToken_TamperedPayload(t *testing.T) {
	_, err := ParseGateToken("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJmYWtlIn0.invalidsignature", testSecret)
	if err == nil {
		t.Error("expected error for tampered token")
	}
}

func TestParseGateToken_EmptyToken(t *testing.T) {
	_, err := ParseGateToken("", testSecret)
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestIssueGateToken_Unique(t *testing.T) {
	gateID := uuid.New()

	t1, err1 := IssueGateToken(gateID, testSecret)
	t2, err2 := IssueGateToken(gateID, testSecret)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v %v", err1, err2)
	}
	if t1 == t2 {
		t.Error("two tokens issued for the same gate should differ (random JTI)")
	}
}

func TestParseGateToken_CorrectTypeParsed(t *testing.T) {
	gateID := uuid.New()
	token, _ := IssueGateToken(gateID, testSecret)

	g, err := ParseGateToken(token, testSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g == uuid.Nil {
		t.Error("parsed gate ID should not be nil")
	}
}
