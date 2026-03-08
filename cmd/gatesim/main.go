// gatesim simulates a physical gate (portail) for development and testing.
//
// Two integration modes:
//
//   - HTTP mode: starts a local HTTP server that gatie calls for open/close commands.
//     After receiving a command, reports the new status back via POST /api/inbound/status.
//     Requires only --token and --api.
//
//   - MQTT mode: connects to the MQTT broker, subscribes to the command topic,
//     responds to open/close commands by publishing a status update.
//     Requires only --token (workspace_id and gate_id are decoded from the JWT).
//
// Both modes send periodic heartbeat status updates to keep the gate alive.
//
// Usage (HTTP mode):
//
//	go run ./cmd/gatesim --token=<gate_token> --mode=http [--api=...] [--listen=:9090]
//
// Usage (MQTT mode):
//
//	go run ./cmd/gatesim --token=<gate_token> --mode=mqtt [--broker=...] [--api=...]
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

func main() {
	token := flag.String("token", "", "Gate JWT token (required)")
	mode := flag.String("mode", "http", "Integration mode: http or mqtt")
	apiURL := flag.String("api", "http://localhost:8080", "Gatie API base URL")
	broker := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	listenAddr := flag.String("listen", ":9090", "Listen address for HTTP mode command server")
	heartbeat := flag.Duration("heartbeat", 10*time.Second, "Heartbeat interval for periodic status updates (0 to disable)")
	flag.Parse()

	if *token == "" {
		slog.Error("--token is required")
		flag.Usage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch *mode {
	case "http":
		sim := &simulator{token: *token, apiURL: *apiURL, status: "closed"}
		runHTTP(ctx, sim, *listenAddr, *heartbeat)

	case "mqtt":
		wsID, gateID, err := parseJWTClaims(*token)
		if err != nil {
			slog.Error("mqtt: cannot decode workspace_id/gate_id from token", "error", err)
			os.Exit(1)
		}
		sim := &simulator{wsID: wsID.String(), gateID: gateID, token: *token, apiURL: *apiURL, status: "closed"}
		runMQTT(ctx, sim, *broker, *heartbeat)

	default:
		slog.Error("unknown mode, use http or mqtt", "mode", *mode)
		os.Exit(1)
	}
}

// parseJWTClaims extracts workspace_id and gate_id (sub) from the JWT payload
// without verifying the signature — gatesim is a dev tool, not a security boundary.
func parseJWTClaims(token string) (wsID, gateID uuid.UUID, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return uuid.Nil, uuid.Nil, fmt.Errorf("not a JWT (expected 3 parts, got %d)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims struct {
		Sub         string `json:"sub"`
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	gateID, err = uuid.Parse(claims.Sub)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse gate_id from sub: %w", err)
	}
	wsID, err = uuid.Parse(claims.WorkspaceID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse workspace_id: %w", err)
	}
	return wsID, gateID, nil
}

// simulator holds shared state.
type simulator struct {
	wsID   string
	gateID uuid.UUID
	token  string
	apiURL string

	mu     sync.Mutex
	status string // current gate status ("open", "closed", etc.)
}

func (s *simulator) getStatus() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *simulator) setStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func sampleMeta(mode string) map[string]any {
	return map[string]any{
		"sim.mode":    mode,
		"sim.latency": "0ms",
		"lora.snr":    float64(-8),
		"lora.rssi":   float64(-72),
	}
}

// ---- MQTT mode ----

func runMQTT(ctx context.Context, sim *simulator, brokerURL string, heartbeatInterval time.Duration) {
	commandTopic := fmt.Sprintf("workspace_%s/gates/%s/command", sim.wsID, sim.gateID)
	statusTopic := fmt.Sprintf("workspace_%s/gates/%s/status", sim.wsID, sim.gateID)

	var mqttClient pahomqtt.Client

	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(fmt.Sprintf("gatesim-%s", sim.gateID)).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second).
		SetOnConnectHandler(func(c pahomqtt.Client) {
			slog.Info("mqtt: connected to broker", "broker", brokerURL)
			t := c.Subscribe(commandTopic, 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
				handleMQTTCommand(ctx, sim, c, msg, statusTopic)
			})
			t.Wait()
			if err := t.Error(); err != nil {
				slog.Error("mqtt: subscribe failed", "error", err)
			} else {
				slog.Info("mqtt: subscribed", "topic", commandTopic)
			}
		}).
		SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			slog.Warn("mqtt: connection lost", "error", err)
		})

	mqttClient = pahomqtt.NewClient(opts)
	if t := mqttClient.Connect(); t.Wait() && t.Error() != nil {
		slog.Error("mqtt: failed to connect", "error", t.Error())
		os.Exit(1)
	}
	defer mqttClient.Disconnect(500)

	// Start heartbeat
	if heartbeatInterval > 0 {
		go mqttHeartbeat(ctx, sim, mqttClient, statusTopic, heartbeatInterval)
	}

	slog.Info("gatesim running in MQTT mode", "workspace_id", sim.wsID, "gate_id", sim.gateID, "heartbeat", heartbeatInterval)
	slog.Info("Waiting for commands (Ctrl+C to stop)")
	<-ctx.Done()
	slog.Info("gatesim stopping")
}

func mqttHeartbeat(ctx context.Context, sim *simulator, client pahomqtt.Client, statusTopic string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			payload := mqttStatusPayload{
				Token:  sim.token,
				Status: sim.getStatus(),
				Meta:   sampleMeta("mqtt"),
			}
			b, _ := json.Marshal(payload)
			t := client.Publish(statusTopic, 1, false, b)
			t.Wait()
			if err := t.Error(); err != nil {
				slog.Warn("mqtt heartbeat: publish failed", "error", err)
			} else {
				slog.Debug("mqtt heartbeat: sent", "status", payload.Status)
			}
		case <-ctx.Done():
			return
		}
	}
}

// commandPayload is the JSON the server sends on the command topic.
type commandPayload struct {
	Action string `json:"action"`
}

// mqttStatusPayload is what this simulator sends on the MQTT status topic.
type mqttStatusPayload struct {
	Token  string         `json:"token"`
	Status string         `json:"status"`
	Meta   map[string]any `json:"meta,omitempty"`
}

func handleMQTTCommand(ctx context.Context, sim *simulator, client pahomqtt.Client, msg pahomqtt.Message, statusTopic string) {
	var cmd commandPayload
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		slog.Warn("mqtt: invalid command payload", "payload", string(msg.Payload()))
		return
	}
	slog.Info("mqtt: received command", "action", cmd.Action)

	newStatus := actionToStatus(cmd.Action)
	sim.setStatus(newStatus)

	payload := mqttStatusPayload{
		Token:  sim.token,
		Status: newStatus,
		Meta:   sampleMeta("mqtt"),
	}
	b, _ := json.Marshal(payload)

	t := client.Publish(statusTopic, 1, false, b)
	t.Wait()
	if err := t.Error(); err != nil {
		slog.Error("mqtt: failed to publish status", "error", err)
		return
	}
	slog.Info("mqtt: published status", "topic", statusTopic, "status", newStatus)
}

// ---- HTTP mode ----

func runHTTP(ctx context.Context, sim *simulator, listenAddr string, heartbeatInterval time.Duration) {
	mux := http.NewServeMux()

	mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		slog.Info("http: received open command")
		sim.setStatus("open")
		w.WriteHeader(http.StatusOK)
		pushStatus(r.Context(), sim, "open", map[string]any{"sim.mode": "http"})
	})

	mux.HandleFunc("/close", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		slog.Info("http: received close command")
		sim.setStatus("closed")
		w.WriteHeader(http.StatusOK)
		pushStatus(r.Context(), sim, "closed", map[string]any{"sim.mode": "http"})
	})

	mux.HandleFunc("/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var cmd commandPayload
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &cmd); err != nil || cmd.Action == "" {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		slog.Info("http: received action command", "action", cmd.Action)
		newStatus := actionToStatus(cmd.Action)
		sim.setStatus(newStatus)
		w.WriteHeader(http.StatusOK)
		pushStatus(r.Context(), sim, newStatus, map[string]any{"sim.mode": "http"})
	})

	srv := &http.Server{Addr: listenAddr, Handler: mux}
	go func() {
		slog.Info("gatesim HTTP command server started", "addr", listenAddr)
		slog.Info("Configure gate open_config url: http://<host>" + listenAddr + "/open")
		slog.Info("Configure gate close_config url: http://<host>" + listenAddr + "/close")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http: server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start heartbeat
	if heartbeatInterval > 0 {
		go httpHeartbeat(ctx, sim, heartbeatInterval)
	}

	slog.Info("gatesim running in HTTP mode", "heartbeat", heartbeatInterval)
	slog.Info("Waiting for commands (Ctrl+C to stop)")
	<-ctx.Done()
	slog.Info("gatesim stopping")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func httpHeartbeat(ctx context.Context, sim *simulator, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status := sim.getStatus()
			pushStatus(ctx, sim, status, map[string]any{"sim.mode": "http"})
			slog.Debug("http heartbeat: sent", "status", status)
		case <-ctx.Done():
			return
		}
	}
}

// pushStatus sends a status update to the gatie inbound API endpoint.
// The gate token in the Authorization header identifies the gate — no gate_id in the URL.
func pushStatus(ctx context.Context, sim *simulator, status string, meta map[string]any) {
	body := map[string]any{"status": status, "meta": meta}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sim.apiURL+"/api/inbound/status", bytes.NewReader(b))
	if err != nil {
		slog.Error("push status: build request failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sim.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("push status: request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("push status: server rejected", "status", resp.StatusCode, "body", string(respBody))
		return
	}
	slog.Info("push status: accepted", "status", status)
}

// actionToStatus maps a command action to a gate status string.
func actionToStatus(action string) string {
	switch action {
	case "open":
		return "open"
	case "close":
		return "closed"
	default:
		return action
	}
}
