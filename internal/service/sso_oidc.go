package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

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
	p, err := gooidc.NewProvider(ctx, issuer)
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
func (s *SSOService) newOIDCProvider(ctx context.Context, settings *SSOSettings, callbackURL string) (*oidcProvider, error) {
	if settings.Issuer == "" || settings.ClientID == "" {
		return nil, ErrSSONotConfigured
	}

	discovered, err := s.oidcCache.get(ctx, settings.Issuer)
	if err != nil {
		return nil, err
	}

	scopes := []string{gooidc.ScopeOpenID, "email", "profile"}
	for _, sc := range settings.Scopes {
		if sc != gooidc.ScopeOpenID && sc != "email" && sc != "profile" {
			scopes = append(scopes, sc)
		}
	}

	return &oidcProvider{
		cfg: &oauth2.Config{
			ClientID:     settings.ClientID,
			ClientSecret: settings.ClientSecret,
			RedirectURL:  callbackURL,
			Endpoint:     discovered.Endpoint(),
			Scopes:       scopes,
		},
		verifier: discovered.Verifier(&gooidc.Config{ClientID: settings.ClientID}),
		svc:      s,
	}, nil
}

func (p *oidcProvider) AuthURL(ctx context.Context) (string, string, error) {
	state, err := newRandomState()
	if err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}
	if err := p.svc.redis.Set(ctx, ssoStatePrefix+state, "1", ssoStateTTL).Err(); err != nil {
		return "", "", fmt.Errorf("store SSO state: %w", err)
	}
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline), state, nil
}

func (p *oidcProvider) Exchange(ctx context.Context, code, state string) (*SSOIdentity, error) {
	// Validate and consume the state token (anti-CSRF).
	n, err := p.svc.redis.Del(ctx, ssoStatePrefix+state).Result()
	if err != nil || n == 0 {
		return nil, ErrSSOInvalidState
	}

	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	email, _ := claims["email"].(string)
	displayName, _ := claims["name"].(string)

	return &SSOIdentity{
		Subject:     idToken.Subject,
		Email:       email,
		DisplayName: displayName,
		Claims:      claims,
	}, nil
}

func newRandomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
