// Package integration contains end-to-end tests that run against a live server.
//
// Prerequisites (run before `task test-integration`):
//
//	task dev-infra   # starts PostgreSQL, Valkey, Mosquitto
//	task dev-api     # starts the Go server on :8080
//
// The test exercises the full gate lifecycle:
//
//  1. Register a platform user
//  2. Create a workspace
//  3. Create a gate with HTTP_INBOUND status push
//  4. Subscribe to SSE and watch events in real-time  (visible in -v output)
//  5. Push gate status updates and verify they arrive via SSE
//  6. Verify status rules override reported status
//  7. Verify token rotation invalidates old token
//  8. Cleanup (delete gate + workspace)
//
// Run with:
//
//	task test-integration
//	# or directly:
//	go test ./tests/integration/... -v -tags integration -timeout 60s
//
//go:build integration

package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── Configuration ────────────────────────────────────────────────────────────

func apiBase() string {
	if u := os.Getenv("GATIE_API_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// ─── HTTP client helper ───────────────────────────────────────────────────────

type testClient struct {
	base    string
	cookies []*http.Cookie
	t       *testing.T
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	return &testClient{base: apiBase(), t: t}
}

func (c *testClient) do(method, path string, body any) *http.Response {
	c.t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, c.base+path, reqBody)
	if err != nil {
		c.t.Fatalf("build request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	c.cookies = append(c.cookies, resp.Cookies()...)
	return resp
}

// call expects the given status and decodes the JSON body.
func (c *testClient) call(method, path string, body any, wantStatus int) map[string]any {
	c.t.Helper()
	resp := c.do(method, path, body)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		c.t.Fatalf("%s %s → %d (want %d)\nbody: %s", method, path, resp.StatusCode, wantStatus, raw)
	}
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func strField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// ─── SSE subscription ─────────────────────────────────────────────────────────

// SSEEvent holds a single parsed SSE event.
type SSEEvent struct {
	Event string
	Data  string
}

// subscribeSSE authenticates, obtains a one-time ticket, then opens the SSE stream.
// Events are forwarded to the returned channel until ctx is cancelled.
//
// SSE routes:
//
//	POST /api/workspaces/{ws_id}/events/ticket  → {"ticket":"..."}
//	GET  /api/workspaces/{ws_id}/events?ticket=...
func subscribeSSE(ctx context.Context, t *testing.T, c *testClient, wsID string) <-chan SSEEvent {
	t.Helper()

	ticketResp := c.do(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/events/ticket", wsID), nil)
	ticketRaw, _ := io.ReadAll(ticketResp.Body)
	ticketResp.Body.Close()
	if ticketResp.StatusCode != http.StatusOK {
		t.Fatalf("SSE ticket: got %d: %s", ticketResp.StatusCode, ticketRaw)
	}
	var ticketBody map[string]any
	json.Unmarshal(ticketRaw, &ticketBody)
	ticket := strField(ticketBody, "ticket")
	if ticket == "" {
		t.Fatalf("SSE ticket response missing 'ticket': %s", ticketRaw)
	}
	t.Logf("[SSE] ticket obtained (%.8s…)", ticket)

	ch := make(chan SSEEvent, 64)
	go func() {
		defer close(ch)

		url := fmt.Sprintf("%s/api/workspaces/%s/events?ticket=%s", c.base, wsID, ticket)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			t.Logf("[SSE] connect error: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("[SSE] stream got %d: %s", resp.StatusCode, body)
			return
		}
		t.Logf("[SSE] stream connected ✓")

		scanner := bufio.NewScanner(resp.Body)
		var ev SSEEvent
		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				ev.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				ev.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			case line == "":
				if ev.Data != "" {
					t.Logf("[SSE] event=%q  data=%s", ev.Event, ev.Data)
					select {
					case ch <- ev:
					default:
					}
				}
				ev = SSEEvent{}
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			t.Logf("[SSE] scanner error: %v", err)
		}
	}()
	return ch
}

// waitForGateStatus blocks until an SSE event carries the expected status for gateID,
// or the timeout fires. Returns the full data string of the matching event.
func waitForGateStatus(t *testing.T, ch <-chan SSEEvent, gateID, wantStatus string, timeout time.Duration) string {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for gate %s status=%q via SSE", gateID, wantStatus)
			return ""
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("SSE channel closed before receiving status=%q", wantStatus)
			}
			var data map[string]any
			if err := json.Unmarshal([]byte(ev.Data), &data); err != nil {
				continue
			}
			if strField(data, "gate_id") == gateID && strField(data, "status") == wantStatus {
				return ev.Data
			}
		}
	}
}

// ─── Status push helper ───────────────────────────────────────────────────────

// pushInbound POSTs a flat JSON payload to /api/inbound/status authenticated by gateToken.
// The payload is: {"status": "<status>", "<meta_key>": <value>, ...}
// Keys in MetaConfig are extracted server-side from the same flat payload.
func pushInbound(t *testing.T, gateToken string, payload map[string]any) int {
	t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		apiBase()+"/api/inbound/status", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gateToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("pushInbound: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Logf("[push] %d: %s  payload=%v", resp.StatusCode, raw, payload)
	} else {
		t.Logf("[push] payload=%v → %d ✓", payload, resp.StatusCode)
	}
	return resp.StatusCode
}

func mustPushInbound(t *testing.T, gateToken string, payload map[string]any) {
	t.Helper()
	code := pushInbound(t, gateToken, payload)
	if code != http.StatusOK && code != http.StatusNoContent {
		t.Fatalf("pushInbound: unexpected status %d", code)
	}
}

// ─── Test bootstrap helpers ───────────────────────────────────────────────────

func section(t *testing.T, label string) {
	t.Helper()
	t.Logf("\n%s\n  %s\n%s", strings.Repeat("─", 60), label, strings.Repeat("─", 60))
}

// bootstrap registers a user (or uses setup/init on first run), creates a workspace,
// and returns (client, workspaceID). Registers cleanup for the workspace.
func bootstrap(t *testing.T) (*testClient, string) {
	t.Helper()
	c := newTestClient(t)

	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	email := fmt.Sprintf("e2e-%s@gatie.test", ts)
	password := "E2eTestP4ss!"

	setupStatus := c.call(http.MethodGet, "/api/setup/status", nil, http.StatusOK)
	if setupStatus["setup_required"] == true {
		t.Logf("[bootstrap] first-run setup with %s", email)
		c.call(http.MethodPost, "/api/setup/init", map[string]any{
			"email": email, "password": password,
		}, http.StatusOK)
	} else {
		c.call(http.MethodPost, "/api/auth/register", map[string]any{
			"email": email, "password": password,
		}, http.StatusOK)
		t.Logf("[bootstrap] registered %s", email)
	}

	wsResp := c.call(http.MethodPost, "/api/workspaces", map[string]any{
		"name": fmt.Sprintf("e2e-ws-%s", ts[:12]),
	}, http.StatusCreated)
	wsID := strField(wsResp, "id")
	if wsID == "" {
		t.Fatalf("[bootstrap] workspace id missing: %v", wsResp)
	}
	t.Logf("[bootstrap] workspace: %s", wsID)

	t.Cleanup(func() {
		t.Logf("[cleanup] deleting workspace %s", wsID)
		c.call(http.MethodDelete, fmt.Sprintf("/api/workspaces/%s", wsID), nil, http.StatusNoContent)
	})
	return c, wsID
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestGateLifecycleLive is the "see it live" test.
// Run with -v to see SSE events and status transitions in real time.
//
// What you will observe in the test output:
//
//	[SSE] stream connected ✓
//	[push] payload=map[battery:85 status:closed] → 204 ✓
//	[SSE] event=""  data={"gate_id":"...","status":"closed",...}
//	[SSE] received status=closed ✓
//	... and so on for each status transition
func TestGateLifecycleLive(t *testing.T) {
	c, wsID := bootstrap(t)

	// ── 0. Health ─────────────────────────────────────────────────────
	section(t, "0 · health check")
	health := c.call(http.MethodGet, "/api/health", nil, http.StatusOK)
	t.Logf("health: db=%s redis=%s", strField(health, "database"), strField(health, "redis"))
	if strField(health, "database") != "ok" || strField(health, "redis") != "ok" {
		t.Fatal("infra not healthy — run `task dev-infra` and `task dev-api` first")
	}

	// ── 1. Create gate ────────────────────────────────────────────────
	section(t, "1 · create gate with HTTP_INBOUND + status rules")
	// status_config: server reads "status" from the top-level key of the pushed JSON body.
	// meta_config:   server extracts "battery" from the same top-level payload.
	// status_rules:  battery < 20 → override status to "low_battery".
	gateResp := c.call(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates", wsID), map[string]any{
		"name":             "e2e-live-gate",
		"integration_type": "NONE",
		"status_config": map[string]any{
			"type": "HTTP_INBOUND",
			"config": map[string]any{
				"mapping": map[string]any{
					"status": map[string]any{
						"field": "status",
					},
				},
			},
		},
		"status_rules": []map[string]any{
			{"key": "battery", "op": "lt", "value": "20", "set_status": "low_battery"},
		},
		"meta_config": []map[string]any{
			{"key": "battery", "label": "Battery", "unit": "%"},
		},
	}, http.StatusCreated)

	gateID := strField(gateResp, "id")
	gateToken := strField(gateResp, "gate_token")
	if gateID == "" {
		t.Fatalf("gate id missing: %v", gateResp)
	}
	if gateToken == "" {
		t.Fatal("gate_token must be present in creation response")
	}
	t.Logf("gate created: id=%s  token=%.20s…", gateID, gateToken)

	t.Cleanup(func() {
		t.Logf("[cleanup] deleting gate %s", gateID)
		c.call(http.MethodDelete, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusNoContent)
	})

	// ── 2. Initial status ─────────────────────────────────────────────
	section(t, "2 · initial status = unknown (never seen)")
	getResp := c.call(http.MethodGet, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusOK)
	if s := strField(getResp, "status"); s != "unknown" {
		t.Errorf("expected unknown, got %q", s)
	} else {
		t.Logf("status=unknown ✓")
	}
	if tok := strField(getResp, "gate_token"); tok != "" {
		t.Errorf("gate_token must NOT appear in GET response, got: %.20s", tok)
	} else {
		t.Logf("gate_token absent in GET ✓")
	}

	// ── 3. Subscribe SSE ──────────────────────────────────────────────
	section(t, "3 · subscribe SSE  ← watch real-time events with -v")
	sseCtx, sseCancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer sseCancel()
	sseCh := subscribeSSE(sseCtx, t, c, wsID)
	// Let the SSE goroutine connect and receive the initial keep-alive comment.
	time.Sleep(400 * time.Millisecond)

	// ── 4. Gate comes online ──────────────────────────────────────────
	section(t, "4 · push status=closed  (gate comes online)")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := waitForGateStatus(t, sseCh, gateID, "closed", 10*time.Second)
		t.Logf("[SSE] received status=closed ✓  %s", data)
	}()
	mustPushInbound(t, gateToken, map[string]any{"status": "closed", "battery": 85.0})
	wg.Wait()

	// ── 5. Gate opens ─────────────────────────────────────────────────
	section(t, "5 · push status=open")
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := waitForGateStatus(t, sseCh, gateID, "open", 10*time.Second)
		t.Logf("[SSE] received status=open   ✓  %s", data)
	}()
	mustPushInbound(t, gateToken, map[string]any{"status": "open", "battery": 84.0})
	wg.Wait()

	// ── 6. Status rule fires ──────────────────────────────────────────
	section(t, "6 · status rule: battery=5 → low_battery override")
	wg.Add(1)
	go func() {
		defer wg.Done()
		data := waitForGateStatus(t, sseCh, gateID, "low_battery", 10*time.Second)
		t.Logf("[SSE] received status=low_battery ✓  %s", data)
	}()
	// Push "closed" with battery=5 — rule overrides to "low_battery".
	mustPushInbound(t, gateToken, map[string]any{"status": "closed", "battery": 5.0})
	wg.Wait()

	// ── 7. DB reflects final status ───────────────────────────────────
	section(t, "7 · confirm DB = low_battery")
	finalResp := c.call(http.MethodGet, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusOK)
	if s := strField(finalResp, "status"); s != "low_battery" {
		t.Errorf("expected DB status=low_battery, got %q", s)
	} else {
		t.Logf("DB status=low_battery ✓")
	}

	// ── 8. Token rotation ─────────────────────────────────────────────
	section(t, "8 · token rotation — old token must be rejected")
	rotateResp := c.call(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates/%s/token/rotate", wsID, gateID), nil, http.StatusOK)
	newToken := strField(rotateResp, "gate_token")
	if newToken == "" {
		t.Fatal("rotate response missing gate_token")
	}
	if newToken == gateToken {
		t.Error("rotated token must differ from the original")
	}
	t.Logf("rotated ✓  new=%.20s…", newToken)

	// Old token → 401
	if code := pushInbound(t, gateToken, map[string]any{"status": "open"}); code != http.StatusUnauthorized {
		t.Errorf("old token should be 401, got %d", code)
	} else {
		t.Logf("old token rejected (401) ✓")
	}

	// New token → accepted
	mustPushInbound(t, newToken, map[string]any{"status": "open", "battery": 80.0})
	t.Logf("new token accepted ✓")

	section(t, "ALL ASSERTIONS PASSED ✓")
}

// TestGateCreateGetDeleteCycle verifies basic CRUD without SSE.
func TestGateCreateGetDeleteCycle(t *testing.T) {
	c, wsID := bootstrap(t)

	health := c.call(http.MethodGet, "/api/health", nil, http.StatusOK)
	if strField(health, "database") != "ok" {
		t.Skip("infra not ready — run `task dev-infra` and `task dev-api`")
	}

	// Create
	gateResp := c.call(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates", wsID), map[string]any{
		"name":             "crud-gate",
		"integration_type": "NONE",
	}, http.StatusCreated)
	gateID := strField(gateResp, "id")
	gateToken := strField(gateResp, "gate_token")
	if gateID == "" {
		t.Fatal("gate id missing")
	}
	if gateToken == "" {
		t.Fatal("gate_token must be present on creation")
	}
	t.Logf("created: id=%s  token=%.20s…", gateID, gateToken)

	// GET — no token, status=unknown
	getResp := c.call(http.MethodGet, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusOK)
	if s := strField(getResp, "status"); s != "unknown" {
		t.Errorf("want status=unknown, got %q", s)
	}
	if tok := strField(getResp, "gate_token"); tok != "" {
		t.Errorf("GET must not return gate_token, got %.20s", tok)
	}
	t.Logf("status=unknown, no token in GET ✓")

	// PATCH rename
	updated := c.call(http.MethodPatch, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID),
		map[string]any{"name": "crud-gate-renamed"}, http.StatusOK)
	if strField(updated, "name") != "crud-gate-renamed" {
		t.Errorf("rename failed, got %q", strField(updated, "name"))
	}
	t.Logf("rename ✓")

	// DELETE
	c.call(http.MethodDelete, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusNoContent)
	t.Logf("delete ✓")

	// GET after delete → 404
	resp := c.do(http.MethodGet, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 after delete, got %d", resp.StatusCode)
	}
	t.Logf("404 after delete ✓")
}

// TestTokenRotation verifies that rotating the gate token invalidates the old one.
func TestTokenRotation(t *testing.T) {
	c, wsID := bootstrap(t)

	health := c.call(http.MethodGet, "/api/health", nil, http.StatusOK)
	if strField(health, "database") != "ok" {
		t.Skip("infra not ready")
	}

	gateResp := c.call(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates", wsID), map[string]any{
		"name":             "token-rotation-gate",
		"integration_type": "NONE",
		"status_config": map[string]any{
			"type": "HTTP_INBOUND",
			"config": map[string]any{
				"mapping": map[string]any{
					"status": map[string]any{"field": "status"},
				},
			},
		},
	}, http.StatusCreated)
	gateID := strField(gateResp, "id")
	oldToken := strField(gateResp, "gate_token")
	t.Cleanup(func() {
		c.call(http.MethodDelete, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusNoContent)
	})

	// Old token works before rotation.
	mustPushInbound(t, oldToken, map[string]any{"status": "closed"})
	t.Logf("old token accepted pre-rotation ✓")

	// Rotate.
	rotateResp := c.call(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates/%s/token/rotate", wsID, gateID), nil, http.StatusOK)
	newToken := strField(rotateResp, "gate_token")
	if newToken == "" || newToken == oldToken {
		t.Fatalf("rotation must return a different non-empty token")
	}
	t.Logf("rotated ✓  new=%.20s…", newToken)

	// Old token → 401
	if code := pushInbound(t, oldToken, map[string]any{"status": "open"}); code != http.StatusUnauthorized {
		t.Errorf("old token: want 401, got %d", code)
	} else {
		t.Logf("old token rejected (401) ✓")
	}

	// New token → 2xx
	mustPushInbound(t, newToken, map[string]any{"status": "open"})
	t.Logf("new token accepted ✓")
}

// TestStatusRulesOverride verifies status rules override raw device status.
func TestStatusRulesOverride(t *testing.T) {
	c, wsID := bootstrap(t)

	health := c.call(http.MethodGet, "/api/health", nil, http.StatusOK)
	if strField(health, "database") != "ok" {
		t.Skip("infra not ready")
	}

	gateResp := c.call(http.MethodPost, fmt.Sprintf("/api/workspaces/%s/gates", wsID), map[string]any{
		"name":             "rule-gate",
		"integration_type": "NONE",
		"status_config": map[string]any{
			"type": "HTTP_INBOUND",
			"config": map[string]any{
				"mapping": map[string]any{
					"status": map[string]any{"field": "status"},
				},
			},
		},
		"status_rules": []map[string]any{
			{"key": "battery", "op": "lt", "value": "20", "set_status": "low_battery"},
			{"key": "battery", "op": "lt", "value": "50", "set_status": "medium_battery"},
		},
	}, http.StatusCreated)
	gateID := strField(gateResp, "id")
	gateToken := strField(gateResp, "gate_token")
	t.Cleanup(func() {
		c.call(http.MethodDelete, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusNoContent)
	})

	cases := []struct {
		battery    float64
		rawStatus  string
		wantStatus string
	}{
		{85.0, "closed", "closed"},         // no rule matches → raw status kept
		{40.0, "closed", "medium_battery"},  // 40 < 50 → second rule
		{10.0, "open", "low_battery"},       // 10 < 20 → first rule wins
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("battery=%.0f", tc.battery), func(t *testing.T) {
			mustPushInbound(t, gateToken, map[string]any{"status": tc.rawStatus, "battery": tc.battery})
			resp := c.call(http.MethodGet, fmt.Sprintf("/api/workspaces/%s/gates/%s", wsID, gateID), nil, http.StatusOK)
			if s := strField(resp, "status"); s != tc.wantStatus {
				t.Errorf("battery=%.0f raw=%q: want %q, got %q", tc.battery, tc.rawStatus, tc.wantStatus, s)
			} else {
				t.Logf("battery=%.0f → status=%q ✓", tc.battery, s)
			}
		})
	}
}
