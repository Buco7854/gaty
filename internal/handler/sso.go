package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Buco7854/gaty/internal/repository"
	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type SSOHandler struct {
	ssoSvc      *service.SSOService
	authSvc     *service.AuthService
	wsRepo      repository.WorkspaceRepository
	frontendURL string
}

func NewSSOHandler(
	ssoSvc *service.SSOService,
	authSvc *service.AuthService,
	wsRepo repository.WorkspaceRepository,
	frontendURL string,
) *SSOHandler {
	return &SSOHandler{ssoSvc: ssoSvc, authSvc: authSvc, wsRepo: wsRepo, frontendURL: frontendURL}
}

// ssoRedirect is the Huma output for HTTP redirect responses.
type ssoRedirect struct {
	Location string `header:"Location"`
}

// --- Public: List SSO providers ---

type SSOProvidersInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

type SSOProvidersOutput struct {
	Body []service.PublicSSOProvider
}

func (h *SSOHandler) ListProviders(ctx context.Context, input *SSOProvidersInput) (*SSOProvidersOutput, error) {
	providers, err := h.ssoSvc.ListPublicProviders(ctx, input.WorkspaceID)
	if errors.Is(err, service.ErrSSONotConfigured) {
		return &SSOProvidersOutput{Body: []service.PublicSSOProvider{}}, nil
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list SSO providers")
	}
	return &SSOProvidersOutput{Body: providers}, nil
}

// --- Authorize ---

type SSOAuthorizeInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	ProviderID  string    `path:"provider_id"`
	GateID      string    `query:"gate_id"`
}

func (h *SSOHandler) Authorize(ctx context.Context, input *SSOAuthorizeInput) (*ssoRedirect, error) {
	authURL, workspaceID, err := h.ssoSvc.GenerateAuthURL(ctx, input.WorkspaceID, input.ProviderID, input.GateID)
	if errors.Is(err, service.ErrSSONotConfigured) {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", input.GateID, workspaceID, "not_configured")}, nil
	}
	if err != nil {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", input.GateID, workspaceID, "config_error")}, nil
	}
	return &ssoRedirect{Location: authURL}, nil
}

// --- Callback ---

type SSOCallbackInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	ProviderID  string    `path:"provider_id"`
	Code        string    `query:"code"`
	State       string    `query:"state"`
	Error       string    `query:"error"`
}

func (h *SSOHandler) Callback(ctx context.Context, input *SSOCallbackInput) (*ssoRedirect, error) {
	if input.Error != "" {
		// Recover gate_id and workspace_id from the Redis state before it expires.
		gateID, workspaceID := h.ssoSvc.ConsumeState(ctx, input.State)
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", gateID, workspaceID, input.Error)}, nil
	}

	membership, gateID, workspaceID, err := h.ssoSvc.Callback(ctx, input.WorkspaceID, input.ProviderID, input.Code, input.State)
	if errors.Is(err, service.ErrSSOInvalidState) {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", "", "", "invalid_state")}, nil
	}
	if errors.Is(err, service.ErrSSOAccessDenied) {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", gateID, workspaceID, "access_denied")}, nil
	}
	if err != nil {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", gateID, workspaceID, "server_error")}, nil
	}

	tokens, err := h.authSvc.IssueLocalTokenPair(ctx, membership.ID, membership.WorkspaceID, membership.Role)
	if err != nil {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", gateID, workspaceID, "server_error")}, nil
	}

	return &ssoRedirect{Location: h.frontendCallbackURL(tokens.AccessToken, tokens.RefreshToken, gateID, workspaceID, "")}, nil
}

func (h *SSOHandler) frontendCallbackURL(accessToken, refreshToken, gateID, workspaceID, errCode string) string {
	u, _ := url.Parse(fmt.Sprintf("%s/auth/sso/callback", h.frontendURL))
	q := u.Query()
	if errCode != "" {
		q.Set("error", errCode)
	}
	if accessToken != "" {
		q.Set("access_token", accessToken)
		q.Set("refresh_token", refreshToken)
	}
	if gateID != "" {
		q.Set("gate_id", gateID)
	}
	if workspaceID != "" {
		q.Set("workspace_id", workspaceID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// --- SSO Settings (admin) ---

type SSOSettingsPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

type SSOSettingsOutput struct {
	Body map[string]any
}

const ssoSecretMask = "***"

// GetSettings returns the SSO settings with client_secret masked.
func (h *SSOHandler) GetSettings(ctx context.Context, input *SSOSettingsPathParam) (*SSOSettingsOutput, error) {
	ws, err := h.wsRepo.GetByID(ctx, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get workspace")
	}
	settings := ws.SSOSettings
	if settings == nil {
		settings = map[string]any{}
	}
	// Mask client_secret in each provider
	settings = maskProviderSecrets(settings)
	return &SSOSettingsOutput{Body: settings}, nil
}

// maskProviderSecrets replaces client_secret in each provider with "***".
func maskProviderSecrets(settings map[string]any) map[string]any {
	providersRaw, ok := settings["providers"]
	if !ok {
		return settings
	}
	// Re-marshal through JSON to work with typed slice
	b, _ := json.Marshal(providersRaw)
	var providers []map[string]any
	if err := json.Unmarshal(b, &providers); err != nil {
		return settings
	}
	for i := range providers {
		if secret, _ := providers[i]["client_secret"].(string); secret != "" {
			providers[i]["client_secret"] = ssoSecretMask
		}
	}
	out := make(map[string]any, len(settings))
	for k, v := range settings {
		out[k] = v
	}
	out["providers"] = providers
	return out
}

type UpdateSSOSettingsInput struct {
	WorkspaceID uuid.UUID      `path:"ws_id"`
	Body        map[string]any
}

// UpdateSettings stores the new SSO settings.
// If a provider's client_secret is "***", the existing secret is preserved.
func (h *SSOHandler) UpdateSettings(ctx context.Context, input *UpdateSSOSettingsInput) (*SSOSettingsOutput, error) {
	body := input.Body

	// Preserve existing client_secret for providers where the value is "***".
	if err := h.preserveSecrets(ctx, input.WorkspaceID, body); err != nil {
		return nil, huma.Error500InternalServerError("failed to preserve existing secrets")
	}

	ws, err := h.wsRepo.UpdateSSOSettings(ctx, input.WorkspaceID, body)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update SSO settings")
	}
	return &SSOSettingsOutput{Body: maskProviderSecrets(ws.SSOSettings)}, nil
}

// preserveSecrets replaces masked "***" client_secret values with the existing ones from DB.
func (h *SSOHandler) preserveSecrets(ctx context.Context, wsID uuid.UUID, body map[string]any) error {
	providersRaw, ok := body["providers"]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(providersRaw)
	var incoming []map[string]any
	if err := json.Unmarshal(b, &incoming); err != nil {
		return nil
	}

	// Check if any need preservation
	needsPreservation := false
	for _, p := range incoming {
		if secret, _ := p["client_secret"].(string); secret == ssoSecretMask {
			needsPreservation = true
			break
		}
	}
	if !needsPreservation {
		return nil
	}

	// Load existing settings
	ws, err := h.wsRepo.GetByID(ctx, wsID)
	if err != nil {
		return err
	}
	existingProviders := map[string]string{} // id → client_secret
	if ws.SSOSettings != nil {
		eb, _ := json.Marshal(ws.SSOSettings["providers"])
		var existing []map[string]any
		if json.Unmarshal(eb, &existing) == nil {
			for _, p := range existing {
				id, _ := p["id"].(string)
				secret, _ := p["client_secret"].(string)
				if id != "" && secret != "" {
					existingProviders[id] = secret
				}
			}
		}
	}

	// Replace "***" with existing values
	for i := range incoming {
		if secret, _ := incoming[i]["client_secret"].(string); secret == ssoSecretMask {
			id, _ := incoming[i]["id"].(string)
			if existing, found := existingProviders[id]; found {
				incoming[i]["client_secret"] = existing
			} else {
				incoming[i]["client_secret"] = ""
			}
		}
	}
	body["providers"] = incoming
	return nil
}

// RegisterRoutes wires SSO endpoints onto the Huma API.
func (h *SSOHandler) RegisterRoutes(
	api huma.API,
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID: "sso-providers",
		Method:      http.MethodGet,
		Path:        "/api/auth/sso/{ws_id}/providers",
		Summary:     "List SSO providers configured for a workspace (public)",
		Tags:        []string{"SSO"},
	}, h.ListProviders)

	huma.Register(api, huma.Operation{
		OperationID:   "sso-authorize",
		Method:        http.MethodGet,
		Path:          "/api/auth/sso/{ws_id}/{provider_id}/authorize",
		Summary:       "Redirect to workspace SSO provider",
		Tags:          []string{"SSO"},
		DefaultStatus: http.StatusFound,
	}, h.Authorize)

	huma.Register(api, huma.Operation{
		OperationID:   "sso-callback",
		Method:        http.MethodGet,
		Path:          "/api/auth/sso/{ws_id}/{provider_id}/callback",
		Summary:       "SSO provider callback — exchanges code for tokens and redirects to frontend",
		Tags:          []string{"SSO"},
		DefaultStatus: http.StatusFound,
	}, h.Callback)

	huma.Register(api, huma.Operation{
		OperationID: "sso-settings-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/sso-settings",
		Summary:     "Get workspace SSO configuration",
		Tags:        []string{"SSO"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.GetSettings)

	huma.Register(api, huma.Operation{
		OperationID: "sso-settings-update",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/sso-settings",
		Summary:     "Update workspace SSO configuration",
		Tags:        []string{"SSO"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.UpdateSettings)
}
