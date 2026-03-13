package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrSSONotConfigured = errors.New("SSO not configured")
	ErrSSOInvalidState  = errors.New("invalid or expired SSO state")
	ErrSSOAccessDenied  = errors.New("SSO access denied: auto-provision disabled")
)

// SSOIdentity holds the normalised identity returned by any SSO provider.
type SSOIdentity struct {
	Subject     string
	Email       string
	DisplayName string
	Claims      map[string]any
}

// SSOProvider is the interface implemented by each concrete SSO backend.
type SSOProvider interface {
	AuthURL(ctx context.Context, gateID, extra string) (url, state string, err error)
	Exchange(ctx context.Context, code, state string) (*SSOIdentity, string, string, error)
}

// SSOProviderConfig holds the configuration for a single SSO provider.
// In the simplified architecture, this comes from environment variables.
type SSOProviderConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "oidc"

	// OIDC fields
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Issuer       string   `json:"issuer"`
	Scopes       []string `json:"scopes"`

	// Manual OAuth2 endpoints
	AuthEndpoint  string `json:"auth_endpoint"`
	TokenEndpoint string `json:"token_endpoint"`
	JwksURL       string `json:"jwks_uri"`

	// Membership provisioning
	AutoProvision bool              `json:"auto_provision"`
	DefaultRole   model.Role        `json:"default_role"`
	RoleClaim     string            `json:"role_claim"`
	RoleMapping   map[string]string `json:"role_mapping"`
}

// PublicSSOProvider is the public representation of an SSO provider (no secrets).
type PublicSSOProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// SSOService orchestrates SSO flows.
// Providers are configured via environment and passed at construction.
type SSOService struct {
	members     repository.MemberRepository
	credentials repository.CredentialRepository
	redis       *redis.Client
	baseURL     string
	providers   []SSOProviderConfig

	oidcCache *oidcDiscoveryCache
}

func NewSSOService(
	members repository.MemberRepository,
	credentials repository.CredentialRepository,
	redisClient *redis.Client,
	baseURL string,
	providers []SSOProviderConfig,
) *SSOService {
	for i := range providers {
		if providers[i].DefaultRole == "" {
			providers[i].DefaultRole = model.RoleMember
		}
	}
	return &SSOService{
		members:     members,
		credentials: credentials,
		redis:       redisClient,
		baseURL:     baseURL,
		providers:   providers,
		oidcCache:   newOIDCDiscoveryCache(),
	}
}

func findProvider(providers []SSOProviderConfig, providerID string) (*SSOProviderConfig, error) {
	for i := range providers {
		if providers[i].ID == providerID {
			return &providers[i], nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", providerID)
}

func (s *SSOService) newProvider(ctx context.Context, cfg *SSOProviderConfig, callbackURL string) (SSOProvider, error) {
	switch cfg.Type {
	case "oidc", "":
		return s.newOIDCProvider(ctx, cfg, callbackURL)
	default:
		return nil, fmt.Errorf("unsupported SSO provider type: %q", cfg.Type)
	}
}

// ListPublicProviders returns the public list of SSO providers (no secrets).
func (s *SSOService) ListPublicProviders() ([]PublicSSOProvider, error) {
	if len(s.providers) == 0 {
		return nil, ErrSSONotConfigured
	}
	public := make([]PublicSSOProvider, len(s.providers))
	for i, p := range s.providers {
		public[i] = PublicSSOProvider{ID: p.ID, Name: p.Name, Type: p.Type}
	}
	return public, nil
}

// GenerateAuthURL builds the SSO authorization URL for the given provider ID.
func (s *SSOService) GenerateAuthURL(ctx context.Context, providerID, gateID string) (authURL string, err error) {
	if len(s.providers) == 0 {
		return "", ErrSSONotConfigured
	}
	cfg, err := findProvider(s.providers, providerID)
	if err != nil {
		return "", ErrSSONotConfigured
	}
	callbackURL := fmt.Sprintf("%s/api/auth/sso/%s/callback", s.baseURL, providerID)
	provider, err := s.newProvider(ctx, cfg, callbackURL)
	if err != nil {
		return "", fmt.Errorf("init SSO provider: %w", err)
	}
	url, _, err := provider.AuthURL(ctx, gateID, "")
	return url, err
}

// Callback processes the SSO callback for the given provider ID.
func (s *SSOService) Callback(ctx context.Context, providerID, code, state string) (*model.Member, string, error) {
	if len(s.providers) == 0 {
		return nil, "", ErrSSONotConfigured
	}
	cfg, err := findProvider(s.providers, providerID)
	if err != nil {
		return nil, "", ErrSSONotConfigured
	}
	callbackURL := fmt.Sprintf("%s/api/auth/sso/%s/callback", s.baseURL, providerID)
	provider, err := s.newProvider(ctx, cfg, callbackURL)
	if err != nil {
		return nil, "", fmt.Errorf("init SSO provider: %w", err)
	}
	identity, gateID, _, err := provider.Exchange(ctx, code, state)
	if err != nil {
		return nil, "", err
	}
	member, err := s.resolveOrProvision(ctx, cfg, identity)
	return member, gateID, err
}

// ConsumeState retrieves and deletes the SSO state from Redis.
func (s *SSOService) ConsumeState(ctx context.Context, state string) (gateID string) {
	if state == "" {
		return ""
	}
	stateJSON, err := s.redis.GetDel(ctx, ssoStatePrefix+state).Result()
	if err != nil {
		return ""
	}
	var stateData ssoState
	_ = json.Unmarshal([]byte(stateJSON), &stateData)
	return stateData.GateID
}

// resolveOrProvision finds or creates the member for a verified SSO identity.
func (s *SSOService) resolveOrProvision(ctx context.Context, cfg *SSOProviderConfig, identity *SSOIdentity) (*model.Member, error) {
	role := cfg.DefaultRole
	if cfg.RoleClaim != "" {
		if claimVal, ok := identity.Claims[cfg.RoleClaim].(string); ok {
			if mapped, found := cfg.RoleMapping[claimVal]; found {
				role = model.Role(mapped)
			}
		}
	}

	existingCred, err := s.credentials.FindBySSOIdentity(ctx, identity.Subject)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("lookup sso identity: %w", err)
	}
	if existingCred != nil {
		return s.members.GetByID(ctx, existingCred.MemberID)
	}

	if !cfg.AutoProvision {
		return nil, ErrSSOAccessDenied
	}

	username := identity.Email
	if username == "" {
		username = identity.Subject
	}
	var displayName *string
	if identity.DisplayName != "" {
		displayName = &identity.DisplayName
	}

	member, err := s.members.Create(ctx, username, displayName, role)
	if errors.Is(err, repository.ErrAlreadyExists) {
		suffix := identity.Subject
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		username = username + "_" + suffix
		member, err = s.members.Create(ctx, username, displayName, role)
	}
	if err != nil {
		return nil, fmt.Errorf("create member: %w", err)
	}

	meta := map[string]any{"email": identity.Email, "issuer": cfg.Issuer}
	_, err = s.credentials.Create(ctx, member.ID, model.CredSSOIdentity, identity.Subject, nil, nil, meta)
	if err != nil {
		return nil, fmt.Errorf("create sso identity credential: %w", err)
	}

	return member, nil
}

// HasProviders returns true if at least one SSO provider is configured.
func (s *SSOService) HasProviders() bool {
	return len(s.providers) > 0
}

// GetProviderByID returns the provider config by ID (for handler-level checks).
func (s *SSOService) GetProviderByID(providerID string) (*SSOProviderConfig, error) {
	return findProvider(s.providers, providerID)
}

// Providers returns the list of configured SSO providers.
func (s *SSOService) Providers() []SSOProviderConfig {
	return s.providers
}

// SetProviders sets SSO providers dynamically (needed for compatibility).
func (s *SSOService) SetProviders(providers []SSOProviderConfig) {
	for i := range providers {
		if providers[i].DefaultRole == "" {
			providers[i].DefaultRole = model.RoleMember
		}
	}
	s.providers = providers
}

// ProviderGateID is a placeholder used when SSO callback associates a gate.
func ProviderGateID(gateID string) uuid.UUID {
	id, _ := uuid.Parse(gateID)
	return id
}
