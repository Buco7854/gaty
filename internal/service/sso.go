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
	ErrSSONotConfigured = errors.New("SSO not configured for this workspace")
	ErrSSOInvalidState  = errors.New("invalid or expired SSO state")
	ErrSSOAccessDenied  = errors.New("SSO access denied: auto-provision disabled")
)

// SSOIdentity holds the normalised identity returned by any SSO provider.
type SSOIdentity struct {
	Subject     string         // unique identifier (sub for OIDC, NameID for SAML, …)
	Email       string
	DisplayName string
	Claims      map[string]any // raw provider claims, used for role mapping
}

// SSOProvider is the interface implemented by each concrete SSO backend (OIDC, SAML, …).
type SSOProvider interface {
	// AuthURL returns the provider's authorization URL and the opaque state token that was stored.
	// gateID and workspaceID are embedded in the Redis state so they survive the cross-origin redirect.
	AuthURL(ctx context.Context, gateID, workspaceID string) (url, state string, err error)
	// Exchange processes the callback parameters, validates the state, and returns the authenticated identity, gateID, and workspaceID.
	Exchange(ctx context.Context, code, state string) (*SSOIdentity, string, string, error)
}

// SSOProviderConfig holds the configuration for a single SSO provider.
// This is stored as an element in the `providers` array in workspaces.sso_settings JSONB.
type SSOProviderConfig struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "oidc" | future: "saml"

	// OIDC fields
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Issuer       string   `json:"issuer"`
	Scopes       []string `json:"scopes"` // additional scopes beyond "openid email profile"

	// Manual OAuth2 endpoints — when set, OIDC auto-discovery is skipped.
	// All three must be provided together for manual mode to work.
	AuthEndpoint  string `json:"auth_endpoint"`
	TokenEndpoint string `json:"token_endpoint"`
	JwksURL       string `json:"jwks_uri"`

	// Membership provisioning
	AutoProvision bool                `json:"auto_provision"`
	DefaultRole   model.WorkspaceRole `json:"default_role"`
	RoleClaim     string              `json:"role_claim"`
	RoleMapping   map[string]string   `json:"role_mapping"`
}

// workspaceSSOSettings is the top-level structure stored in workspaces.sso_settings JSONB.
type workspaceSSOSettings struct {
	Providers []SSOProviderConfig `json:"providers"`
}

// PublicSSOProvider is the public representation of an SSO provider (no secrets).
type PublicSSOProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func parseWorkspaceSSOSettings(raw map[string]any) ([]SSOProviderConfig, error) {
	if len(raw) == 0 {
		return nil, ErrSSONotConfigured
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal sso settings: %w", err)
	}
	var ws workspaceSSOSettings
	if err := json.Unmarshal(b, &ws); err != nil {
		return nil, fmt.Errorf("parse sso settings: %w", err)
	}
	if len(ws.Providers) == 0 {
		return nil, ErrSSONotConfigured
	}
	for i := range ws.Providers {
		if ws.Providers[i].DefaultRole == "" {
			ws.Providers[i].DefaultRole = model.RoleMember
		}
	}
	return ws.Providers, nil
}

// SSOService orchestrates workspace SSO flows.
// It is provider-agnostic: concrete backends are instantiated by newProvider.
type SSOService struct {
	workspaces  repository.WorkspaceRepository
	memberships repository.WorkspaceMembershipRepository
	memberCreds repository.MembershipCredentialRepository
	redis       *redis.Client
	baseURL     string

	// OIDC discovery cache — defined in sso_oidc.go, shared across all OIDC provider instances.
	oidcCache *oidcDiscoveryCache
}

func NewSSOService(
	workspaces repository.WorkspaceRepository,
	memberships repository.WorkspaceMembershipRepository,
	memberCreds repository.MembershipCredentialRepository,
	redisClient *redis.Client,
	baseURL string,
) *SSOService {
	return &SSOService{
		workspaces:  workspaces,
		memberships: memberships,
		memberCreds: memberCreds,
		redis:       redisClient,
		baseURL:     baseURL,
		oidcCache:   newOIDCDiscoveryCache(),
	}
}

// findProvider returns the provider with the given ID from the settings slice.
func findProvider(providers []SSOProviderConfig, providerID string) (*SSOProviderConfig, error) {
	for i := range providers {
		if providers[i].ID == providerID {
			return &providers[i], nil
		}
	}
	return nil, fmt.Errorf("provider %q not found", providerID)
}

// newProvider instantiates the concrete SSOProvider for the given config.
func (s *SSOService) newProvider(ctx context.Context, cfg *SSOProviderConfig, callbackURL string) (SSOProvider, error) {
	switch cfg.Type {
	case "oidc", "":
		return s.newOIDCProvider(ctx, cfg, callbackURL)
	default:
		return nil, fmt.Errorf("unsupported SSO provider type: %q", cfg.Type)
	}
}

// ListPublicProviders returns the public list of SSO providers for a workspace (no secrets).
func (s *SSOService) ListPublicProviders(ctx context.Context, workspaceID uuid.UUID) ([]PublicSSOProvider, error) {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrSSONotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	providers, err := parseWorkspaceSSOSettings(ws.SSOSettings)
	if err != nil {
		return nil, err
	}
	public := make([]PublicSSOProvider, len(providers))
	for i, p := range providers {
		public[i] = PublicSSOProvider{ID: p.ID, Name: p.Name, Type: p.Type}
	}
	return public, nil
}

// GenerateAuthURL builds the SSO authorization URL for the given workspace and provider ID.
// gateID and workspaceID are stored in the Redis state so they can be recovered during the callback.
// Returns the auth URL and the workspace ID (for use in error redirects).
func (s *SSOService) GenerateAuthURL(ctx context.Context, workspaceID uuid.UUID, providerID, gateID string) (authURL string, wsID string, err error) {
	ws, werr := s.workspaces.GetByID(ctx, workspaceID)
	if errors.Is(werr, repository.ErrNotFound) {
		return "", "", ErrSSONotConfigured
	}
	if werr != nil {
		return "", "", fmt.Errorf("get workspace: %w", werr)
	}
	providers, werr := parseWorkspaceSSOSettings(ws.SSOSettings)
	if werr != nil {
		return "", ws.ID.String(), werr
	}
	cfg, werr := findProvider(providers, providerID)
	if werr != nil {
		return "", ws.ID.String(), ErrSSONotConfigured
	}
	callbackURL := fmt.Sprintf("%s/api/auth/sso/%s/%s/callback", s.baseURL, ws.ID.String(), providerID)
	provider, werr := s.newProvider(ctx, cfg, callbackURL)
	if werr != nil {
		return "", ws.ID.String(), fmt.Errorf("init SSO provider: %w", werr)
	}
	url, _, werr := provider.AuthURL(ctx, gateID, ws.ID.String())
	return url, ws.ID.String(), werr
}

// Callback processes the SSO callback for the given workspace and provider ID.
// Returns the resolved membership, the gateID, and the workspaceID embedded in the state.
func (s *SSOService) Callback(ctx context.Context, workspaceID uuid.UUID, providerID, code, state string) (*model.WorkspaceMembership, string, string, error) {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, "", "", ErrSSONotConfigured
	}
	if err != nil {
		return nil, "", "", fmt.Errorf("get workspace: %w", err)
	}
	providers, err := parseWorkspaceSSOSettings(ws.SSOSettings)
	if err != nil {
		return nil, "", "", err
	}
	cfg, err := findProvider(providers, providerID)
	if err != nil {
		return nil, "", "", ErrSSONotConfigured
	}
	callbackURL := fmt.Sprintf("%s/api/auth/sso/%s/%s/callback", s.baseURL, ws.ID.String(), providerID)
	provider, err := s.newProvider(ctx, cfg, callbackURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("init SSO provider: %w", err)
	}
	identity, gateID, _, err := provider.Exchange(ctx, code, state)
	if err != nil {
		return nil, "", "", err
	}
	membership, err := s.resolveOrProvision(ctx, ws.ID, cfg, identity)
	return membership, gateID, ws.ID.String(), err
}

// ConsumeState retrieves and deletes the SSO state from Redis (for OAuth error paths).
// Returns empty strings if the state is missing or invalid.
func (s *SSOService) ConsumeState(ctx context.Context, state string) (gateID, workspaceID string) {
	if state == "" {
		return "", ""
	}
	stateJSON, err := s.redis.GetDel(ctx, ssoStatePrefix+state).Result()
	if err != nil {
		return "", ""
	}
	var stateData ssoState
	_ = json.Unmarshal([]byte(stateJSON), &stateData)
	return stateData.GateID, stateData.WorkspaceID
}

// resolveOrProvision finds or creates the workspace membership for a verified SSO identity.
func (s *SSOService) resolveOrProvision(ctx context.Context, workspaceID uuid.UUID, cfg *SSOProviderConfig, identity *SSOIdentity) (*model.WorkspaceMembership, error) {
	role := cfg.DefaultRole
	if cfg.RoleClaim != "" {
		if claimVal, ok := identity.Claims[cfg.RoleClaim].(string); ok {
			if mapped, found := cfg.RoleMapping[claimVal]; found {
				role = model.WorkspaceRole(mapped)
			}
		}
	}

	existingCred, err := s.memberCreds.FindBySSOIdentity(ctx, workspaceID, identity.Subject)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("lookup sso identity: %w", err)
	}
	if existingCred != nil {
		return s.memberships.GetByID(ctx, existingCred.MembershipID, workspaceID)
	}

	if !cfg.AutoProvision {
		return nil, ErrSSOAccessDenied
	}

	localUsername := identity.Email
	if localUsername == "" {
		localUsername = identity.Subject
	}
	var displayName *string
	if identity.DisplayName != "" {
		displayName = &identity.DisplayName
	}

	membership, err := s.memberships.CreateLocal(ctx, workspaceID, localUsername, displayName, role, nil)
	if errors.Is(err, repository.ErrAlreadyExists) {
		suffix := identity.Subject
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		localUsername = localUsername + "_" + suffix
		membership, err = s.memberships.CreateLocal(ctx, workspaceID, localUsername, displayName, role, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("create membership: %w", err)
	}

	meta := map[string]any{"email": identity.Email, "issuer": cfg.Issuer}
	_, err = s.memberCreds.Create(ctx, membership.ID, model.CredSSOIdentity, identity.Subject, nil, nil, meta)
	if err != nil {
		return nil, fmt.Errorf("create sso identity credential: %w", err)
	}

	return membership, nil
}
