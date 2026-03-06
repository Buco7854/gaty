package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/model"
)

// httpClient is a dedicated client for gate HTTP drivers.
// Using a private client (rather than http.DefaultClient) isolates gate traffic
// and ensures TCP/TLS-level timeouts independent of the request context.
var httpClient = &http.Client{
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

// HTTPDriver sends an HTTP request to a configured URL when triggered.
type HTTPDriver struct {
	url     string
	method  string
	headers map[string]string
	body    string
}

// NewHTTPDriver builds an HTTPDriver from an ActionConfig's config map.
// Required key: "url". Optional: "method" (default POST), "headers", "body".
func NewHTTPDriver(cfg map[string]any) (*HTTPDriver, error) {
	url, _ := cfg["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("http driver: missing required field 'url'")
	}
	method := "POST"
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
	return &HTTPDriver{url: url, method: method, headers: headers, body: body}, nil
}

func (d *HTTPDriver) Execute(ctx context.Context, _ *model.Gate) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if d.body != "" {
		bodyReader = bytes.NewBufferString(d.body)
	}

	req, err := http.NewRequestWithContext(ctx, d.method, d.url, bodyReader)
	if err != nil {
		return fmt.Errorf("http driver: build request: %w", err)
	}
	for k, v := range d.headers {
		req.Header.Set(k, v)
	}
	if d.body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http driver: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("http driver: server returned %d", resp.StatusCode)
	}
	return nil
}
