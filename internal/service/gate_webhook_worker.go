package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/redis/go-redis/v9"
)

const webhookCheckInterval = 10 * time.Second

// maxConcurrentPolls limits the number of simultaneous outbound HTTP webhook requests
// to prevent resource exhaustion when many gates are due at the same time.
const maxConcurrentPolls = 10

// GateWebhookWorker periodically polls all HTTP_WEBHOOK-configured gates for their status.
// It respects the per-gate interval_seconds setting and retries failed requests according
// to the global max_retries / retry_delay configuration.
type GateWebhookWorker struct {
	gates      repository.GateRepository
	redis      *redis.Client
	maxRetries int
	retryDelay time.Duration
	httpClient *http.Client
}

// NewGateWebhookWorker creates a worker with the given retry settings.
// httpClient should be built with SSRF protection; pass nil to use a safe default.
func NewGateWebhookWorker(gates repository.GateRepository, redis *redis.Client, maxRetries int, retryDelay time.Duration, httpClient *http.Client) *GateWebhookWorker {
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				MaxIdleConns:          32,
				MaxIdleConnsPerHost:   4,
				IdleConnTimeout:       90 * time.Second,
			},
		}
	}
	return &GateWebhookWorker{
		gates:      gates,
		redis:      redis,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		httpClient: httpClient,
	}
}

// Run starts the webhook polling loop. It blocks until ctx is cancelled.
func (w *GateWebhookWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(webhookCheckInterval)
	defer ticker.Stop()

	slog.Info("gate webhook worker started", "check_interval", webhookCheckInterval,
		"max_retries", w.maxRetries, "retry_delay", w.retryDelay)

	for {
		select {
		case <-ticker.C:
			w.pollAll(ctx)
		case <-ctx.Done():
			slog.Info("gate webhook worker stopped")
			return
		}
	}
}

func (w *GateWebhookWorker) pollAll(ctx context.Context) {
	tCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	gates, err := w.gates.ListWebhookGates(tCtx)
	if err != nil {
		slog.Error("webhook: failed to list webhook gates", "error", err)
		return
	}

	sem := make(chan struct{}, maxConcurrentPolls)
	var wg sync.WaitGroup

	for i := range gates {
		g := &gates[i]
		if !isDue(g) {
			continue
		}

		wg.Add(1)
		go func(gate *model.Gate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			w.pollGate(pollCtx, gate)
		}(g)
	}

	wg.Wait()
}

// isDue reports whether a gate is due for a webhook poll based on its configured
// interval_seconds (default 60s) and its last_seen_at timestamp.
func isDue(gate *model.Gate) bool {
	interval := 60 * time.Second
	if gate.StatusConfig != nil && len(gate.StatusConfig.Config) > 0 {
		if v, ok := gate.StatusConfig.Config["interval_seconds"]; ok {
			switch n := v.(type) {
			case float64:
				interval = time.Duration(n) * time.Second
			case int:
				interval = time.Duration(n) * time.Second
			}
		}
	}
	if gate.LastSeenAt == nil {
		return true
	}
	return time.Since(*gate.LastSeenAt) >= interval
}

func (w *GateWebhookWorker) pollGate(ctx context.Context, gate *model.Gate) {
	cfg := gate.StatusConfig.Config

	url, _ := cfg["url"].(string)
	if url == "" {
		slog.Warn("webhook: gate has no url configured", "gate_id", gate.ID)
		return
	}

	method := "GET"
	if m, ok := cfg["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	headers := map[string]string{}
	if h, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}

	body, _ := cfg["body"].(string)

	mapping, ok := model.ExtractPayloadMapping(cfg)
	if !ok {
		slog.Warn("webhook: gate has no payload mapping configured", "gate_id", gate.ID)
		return
	}

	successRanges := model.ExtractSuccessStatusCodes(cfg)

	raw, err := w.fetchWithRetry(ctx, method, url, headers, body, successRanges)
	if err != nil {
		slog.Warn("webhook: poll failed", "gate_id", gate.ID, "url", url, "error", err)
		return
	}

	status, err := model.ApplyMapping(*mapping, raw)
	if err != nil {
		slog.Warn("webhook: payload mapping failed", "gate_id", gate.ID, "error", err)
		return
	}
	meta := model.ExtractMeta(gate.MetaConfig, raw)

	if err := ProcessGateStatus(ctx, w.gates, w.redis, gate, status, meta); err != nil {
		slog.Error("webhook: failed to process gate status", "gate_id", gate.ID, "error", err)
	}
}

// fetchWithRetry performs the HTTP request and retries up to maxRetries times on failure.
// Returns the parsed JSON response body as map[string]any.
func (w *GateWebhookWorker) fetchWithRetry(ctx context.Context, method, url string, headers map[string]string, body string, successRanges []model.StatusCodeRange) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(w.retryDelay):
			}
		}

		result, err := fetchJSON(ctx, w.httpClient, method, url, headers, body, successRanges)
		if err == nil {
			return result, nil
		}
		lastErr = err
		slog.Debug("webhook: poll attempt failed", "attempt", attempt+1, "error", err)
	}
	return nil, fmt.Errorf("all %d attempts failed: %w", w.maxRetries+1, lastErr)
}

func fetchJSON(ctx context.Context, client *http.Client, method, url string, headers map[string]string, body string, successRanges []model.StatusCodeRange) (map[string]any, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if !model.IsSuccessStatus(resp.StatusCode, successRanges) {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode JSON response: %w", err)
	}
	return result, nil
}
