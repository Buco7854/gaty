package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/model"
)

// privateIPNets lists CIDR blocks that must never be reached by the HTTP driver.
// This prevents SSRF attacks where a gate config points to internal infrastructure.
var privateIPNets []*net.IPNet

func init() {
	blocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // link-local / AWS metadata
		"100.64.0.0/10",  // shared address space (RFC 6598)
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, block := range blocks {
		_, ipNet, _ := net.ParseCIDR(block)
		privateIPNets = append(privateIPNets, ipNet)
	}
}

// isPrivateIP reports whether addr (an IP address string) falls in a private range.
func isPrivateIP(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	for _, block := range privateIPNets {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// isAllowed reports whether addr falls within one of the explicitly allowed CIDRs.
func isAllowed(addr string, cidrs []*net.IPNet) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	for _, cidr := range cidrs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ssrfSafeDialContext wraps a net.Dialer and rejects connections to private IPs,
// unless the resolved IP is covered by one of the explicitly allowed CIDRs.
func ssrfSafeDialContext(allowedCIDRs []*net.IPNet) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("ssrf check: %w", err)
		}
		// Resolve hostname to IPs and reject any private address not in the allowlist.
		addrs, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("ssrf check: resolve %s: %w", host, err)
		}
		for _, a := range addrs {
			if isPrivateIP(a) && !isAllowed(a, allowedCIDRs) {
				return nil, fmt.Errorf("http driver: target resolves to a private/reserved IP (%s), blocked for security", a)
			}
		}
		return dialer.DialContext(ctx, network, addr)
	}
}

// NewHTTPClient builds a dedicated HTTP client for gate drivers with SSRF protection
// (private/reserved IPs are blocked at the dial level).
// allowedCIDRs lists subnets exempt from the block (e.g. an on-prem gate LAN).
func NewHTTPClient(allowedCIDRs []*net.IPNet) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           ssrfSafeDialContext(allowedCIDRs),
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          32,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

// HTTPDriver sends an HTTP request to a configured URL when triggered.
type HTTPDriver struct {
	url     string
	method  string
	headers map[string]string
	body    string
	client  *http.Client
}

// NewHTTPDriver builds an HTTPDriver from an ActionConfig's config map.
// Required key: "url". Optional: "method" (default POST), "headers", "body".
// client must be built via NewHTTPClient to ensure SSRF protection is in place.
func NewHTTPDriver(cfg map[string]any, client *http.Client) (*HTTPDriver, error) {
	rawURL, _ := cfg["url"].(string)
	if rawURL == "" {
		return nil, fmt.Errorf("http driver: missing required field 'url'")
	}

	// Reject obviously non-HTTP schemes before even dialing.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("http driver: invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("http driver: unsupported scheme %q (only http/https allowed)", parsed.Scheme)
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
	return &HTTPDriver{url: rawURL, method: method, headers: headers, body: body, client: client}, nil
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

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("http driver: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("http driver: server returned %d", resp.StatusCode)
	}
	return nil
}
