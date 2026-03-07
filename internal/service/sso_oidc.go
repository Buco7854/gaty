package service

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// oidcHTTPClient forces HTTP/1.1 to avoid issues with HTTP/2 on some OIDC providers.
var oidcHTTPClient = &http.Client{
	Transport: &http.Transport{
		TLSNextProto:    make(map[string]func(string, *tls.Conn) http.RoundTripper),
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	},
}

type ssoState struct {
	GateID      string `json:"gate_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

const (
	ssoStateTTL    = 10 * time.Minute
	ssoStatePrefix = "sso:state:"
)

// oidcDiscoveryCache caches OIDC provider discovery documents by issuer URL,
// avoiding repeated HTTP calls to the well-known endpoint.
type oidcDiscoveryCache struct {
	mu    sync.RWMutex
	store map[string]*gooidc.Provider
}

func newOIDCDiscoveryCache() *oidcDiscoveryCache {
	return &oidcDiscoveryCache{store: make(map[string]*gooidc.Provider)}
}

func (c *oidcDiscoveryCache) get(ctx context.Context, issuer string) (*gooidc.Provider, error) {
	c.mu.RLock()
	p, ok := c.store[issuer]
	c.mu.RUnlock()
	if ok {
		return p, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok = c.store[issuer]; ok {
		return p, nil
	}
	p, err := gooidc.NewProvider(gooidc.ClientContext(ctx, oidcHTTPClient), issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery for %q: %w", issuer, err)
	}
	c.store[issuer] = p
	return p, nil
}

// oidcProvider implements SSOProvider for OpenID Connect (RFC 6749 + OIDC Core).
type oidcProvider struct {
	cfg      *oauth2.Config
	verifier *gooidc.IDTokenVerifier
	svc      *SSOService // for Redis state management
}

// newOIDCProvider is a factory method on *SSOService so it can share the discovery cache.
func (s *SSOService) newOIDCProvider(ctx context.Context, cfg *SSOProviderConfig, callbackURL string) (*oidcProvider, error) {
	if cfg.ClientID == "" {
		return nil, ErrSSONotConfigured
	}

	scopes := []string{gooidc.ScopeOpenID, "email", "profile"}
	for _, sc := range cfg.Scopes {
		if sc != gooidc.ScopeOpenID && sc != "email" && sc != "profile" {
			scopes = append(scopes, sc)
		}
	}

	// Manual mode: skip OIDC auto-discovery when custom endpoints are provided.
	if cfg.AuthEndpoint != "" && cfg.TokenEndpoint != "" {
		if cfg.JwksURL == "" {
			return nil, fmt.Errorf("jwks_uri is required when using manual OAuth2 endpoints")
		}
		keySet := gooidc.NewRemoteKeySet(ctx, cfg.JwksURL)
		issuer := cfg.Issuer
		if issuer == "" {
			issuer = cfg.AuthEndpoint // fallback for issuer claim validation
		}
		return &oidcProvider{
			cfg: &oauth2.Config{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				RedirectURL:  callbackURL,
				Endpoint:     oauth2.Endpoint{AuthURL: cfg.AuthEndpoint, TokenURL: cfg.TokenEndpoint},
				Scopes:       scopes,
			},
			verifier: gooidc.NewVerifier(issuer, keySet, &gooidc.Config{ClientID: cfg.ClientID}),
			svc:      s,
		}, nil
	}

	// Auto-discovery mode (standard OIDC).
	if cfg.Issuer == "" {
		return nil, ErrSSONotConfigured
	}
	discovered, err := s.oidcCache.get(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}

	return &oidcProvider{
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  callbackURL,
			Endpoint:     discovered.Endpoint(),
			Scopes:       scopes,
		},
		verifier: discovered.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		svc:      s,
	}, nil
}

func (p *oidcProvider) AuthURL(ctx context.Context, gateID, workspaceID string) (string, string, error) {
	state, err := newRandomState()
	if err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}
	stateJSON, err := json.Marshal(ssoState{GateID: gateID, WorkspaceID: workspaceID})
	if err != nil {
		return "", "", fmt.Errorf("marshal state: %w", err)
	}
	if err := p.svc.redis.Set(ctx, ssoStatePrefix+state, string(stateJSON), ssoStateTTL).Err(); err != nil {
		return "", "", fmt.Errorf("store SSO state: %w", err)
	}
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), state, nil
}

func (p *oidcProvider) Exchange(ctx context.Context, code, state string) (*SSOIdentity, string, string, error) {
	// Retrieve and consume the state token (anti-CSRF).
	stateJSON, err := p.svc.redis.GetDel(ctx, ssoStatePrefix+state).Result()
	if err != nil {
		return nil, "", "", ErrSSOInvalidState
	}
	var stateData ssoState
	_ = json.Unmarshal([]byte(stateJSON), &stateData)

	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, "", "", fmt.Errorf("exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, "", "", fmt.Errorf("no id_token in response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", "", fmt.Errorf("verify id_token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, "", "", fmt.Errorf("extract claims: %w", err)
	}

	email, _ := claims["email"].(string)
	displayName, _ := claims["name"].(string)

	return &SSOIdentity{
		Subject:     idToken.Subject,
		Email:       email,
		DisplayName: displayName,
		Claims:      claims,
	}, stateData.GateID, stateData.WorkspaceID, nil
}

func newRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
