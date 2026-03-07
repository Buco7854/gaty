package safenet

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// allowedHTTPMethods is the set of HTTP methods allowed for user-configured requests.
var allowedHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// privateRanges contains CIDR ranges that must be blocked for outbound requests
// to prevent SSRF attacks (loopback, link-local, private RFC1918, cloud metadata, etc.).
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // link-local / cloud metadata (AWS, GCP, Azure)
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	} {
		_, n, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, n)
	}
}

// isPrivateIP returns true if the IP falls within a blocked range.
func isPrivateIP(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL checks that a URL is safe for outbound requests:
//   - scheme must be http or https
//   - hostname must not resolve to a private/loopback IP
//   - port is optional (defaults to scheme default)
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL scheme %q is not allowed (only http and https)", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	// Resolve hostname to IPs and check each one.
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve hostname %q: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to a private/internal IP address")
		}
	}
	return nil
}

// ValidateHTTPMethod checks that a method string is in the allowed set.
func ValidateHTTPMethod(method string) error {
	if !allowedHTTPMethods[strings.ToUpper(method)] {
		return fmt.Errorf("HTTP method %q is not allowed (allowed: GET, POST, PUT, PATCH, DELETE)", method)
	}
	return nil
}
