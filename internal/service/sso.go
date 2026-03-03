package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
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
	AuthURL(ctx context.Context) (url, state string, err error)
	// Exchange processes the callback parameters, validates the state, and returns the authenticated identity.
	Exchange(ctx context.Context, code, state string) (*SSOIdentity, error)
}

// SSOSettings holds the SSO configuration stored in workspaces.sso_settings JSONB.
// Common fields are defined here; provider-specific fields are read by each provider.
type SSOSettings struct {
	Provider string `json:"provider"` // "oidc" (default), "saml", …

	// OIDC fields
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Issuer       string   `json:"issuer"`
	Scopes       []string `json:"scopes"` // additional scopes beyond "openid"

	// Membership provisioning
	AutoProvision bool                `json:"auto_provision"`
	DefaultRole   model.WorkspaceRole `json:"default_role"`
	RoleClaim     string              `json:"role_claim"`
	RoleMapping   map[string]string   `json:"role_mapping"`
}

func parseSSOSettings(raw map[string]any) (*SSOSettings, error) {
	if len(raw) == 0 {
		return nil, ErrSSONotConfigured
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal sso settings: %w", err)
	}
	var s SSOSettings
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse sso settings: %w", err)
	}
	if s.DefaultRole == "" {
		s.DefaultRole = model.RoleMember
	}
	return &s, nil
}

// SSOService orchestrates workspace SSO flows.
// It is provider-agnostic: concrete backends are instantiated by newProvider.
type SSOService struct {
	workspaces  *repository.WorkspaceRepository
	memberships *repository.WorkspaceMembershipRepository
	memberCreds *repository.MembershipCredentialRepository
	redis       *redis.Client
	baseURL     string

	// OIDC discovery cache — defined in sso_oidc.go, shared across all OIDC provider instances.
	oidcCache *oidcDiscoveryCache
}

func NewSSOService(
	workspaces *repository.WorkspaceRepository,
	memberships *repository.WorkspaceMembershipRepository,
	memberCreds *repository.MembershipCredentialRepository,
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

// newProvider instantiates the concrete SSOProvider for the given settings and workspace slug.
func (s *SSOService) newProvider(ctx context.Context, settings *SSOSettings, workspaceSlug string) (SSOProvider, error) {
	callbackURL := fmt.Sprintf("%s/api/auth/sso/%s/callback", s.baseURL, workspaceSlug)
	switch settings.Provider {
	case "oidc", "":
		return s.newOIDCProvider(ctx, settings, callbackURL)
	default:
		return nil, fmt.Errorf("unsupported SSO provider: %q", settings.Provider)
	}
}

// GenerateAuthURL builds the SSO authorization URL for the given workspace slug.
func (s *SSOService) GenerateAuthURL(ctx context.Context, workspaceSlug string) (string, error) {
	ws, err := s.workspaces.GetBySlug(ctx, workspaceSlug)
	if errors.Is(err, repository.ErrNotFound) {
		return "", ErrSSONotConfigured
	}
	if err != nil {
		return "", fmt.Errorf("get workspace: %w", err)
	}

	settings, err := parseSSOSettings(ws.SSOSettings)
	if err != nil {
		return "", err
	}

	provider, err := s.newProvider(ctx, settings, workspaceSlug)
	if err != nil {
		return "", fmt.Errorf("init SSO provider: %w", err)
	}

	authURL, _, err := provider.AuthURL(ctx)
	return authURL, err
}

// Callback processes the SSO callback for the given workspace slug.
// Returns the resolved (or newly created) workspace membership.
func (s *SSOService) Callback(ctx context.Context, workspaceSlug, code, state string) (*model.WorkspaceMembership, error) {
	ws, err := s.workspaces.GetBySlug(ctx, workspaceSlug)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrSSONotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}

	settings, err := parseSSOSettings(ws.SSOSettings)
	if err != nil {
		return nil, err
	}

	provider, err := s.newProvider(ctx, settings, workspaceSlug)
	if err != nil {
		return nil, fmt.Errorf("init SSO provider: %w", err)
	}

	identity, err := provider.Exchange(ctx, code, state)
	if err != nil {
		return nil, err
	}

	return s.resolveOrProvision(ctx, ws.ID, settings, identity)
}

// resolveOrProvision finds or creates the workspace membership for a verified SSO identity.
// This logic is provider-agnostic: it only depends on SSOIdentity and SSOSettings.
func (s *SSOService) resolveOrProvision(ctx context.Context, workspaceID uuid.UUID, settings *SSOSettings, identity *SSOIdentity) (*model.WorkspaceMembership, error) {
	role := settings.DefaultRole
	if settings.RoleClaim != "" {
		if claimVal, ok := identity.Claims[settings.RoleClaim].(string); ok {
			if mapped, found := settings.RoleMapping[claimVal]; found {
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

	if !settings.AutoProvision {
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

	meta := map[string]any{"email": identity.Email, "issuer": settings.Issuer}
	_, err = s.memberCreds.Create(ctx, membership.ID, model.CredSSOIdentity, identity.Subject, nil, nil, meta)
	if err != nil {
		return nil, fmt.Errorf("create sso identity credential: %w", err)
	}

	return membership, nil
}
