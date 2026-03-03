package handler

import (
	"context"
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
	wsRepo      *repository.WorkspaceRepository
	frontendURL string
}

func NewSSOHandler(
	ssoSvc *service.SSOService,
	authSvc *service.AuthService,
	wsRepo *repository.WorkspaceRepository,
	frontendURL string,
) *SSOHandler {
	return &SSOHandler{ssoSvc: ssoSvc, authSvc: authSvc, wsRepo: wsRepo, frontendURL: frontendURL}
}

// ssoRedirect is the Huma output for HTTP redirect responses.
type ssoRedirect struct {
	Location string `header:"Location"`
}

// --- Authorize ---

type SSOAuthorizeInput struct {
	WorkspaceSlug string `path:"ws_slug"`
}

func (h *SSOHandler) Authorize(ctx context.Context, input *SSOAuthorizeInput) (*ssoRedirect, error) {
	authURL, err := h.ssoSvc.GenerateAuthURL(ctx, input.WorkspaceSlug)
	if errors.Is(err, service.ErrSSONotConfigured) {
		return nil, huma.Error404NotFound("SSO not configured for this workspace")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to build SSO URL")
	}
	return &ssoRedirect{Location: authURL}, nil
}

// --- Callback ---

type SSOCallbackInput struct {
	WorkspaceSlug string `path:"ws_slug"`
	Code          string `query:"code"`
	State         string `query:"state"`
	Error         string `query:"error"`
}

func (h *SSOHandler) Callback(ctx context.Context, input *SSOCallbackInput) (*ssoRedirect, error) {
	// Provider reported an error (e.g., user denied access).
	if input.Error != "" {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", input.Error)}, nil
	}

	membership, err := h.ssoSvc.Callback(ctx, input.WorkspaceSlug, input.Code, input.State)
	if errors.Is(err, service.ErrSSOInvalidState) {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", "invalid_state")}, nil
	}
	if errors.Is(err, service.ErrSSOAccessDenied) {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", "access_denied")}, nil
	}
	if err != nil {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", "server_error")}, nil
	}

	tokens, err := h.authSvc.IssueLocalTokenPair(ctx, membership.ID, membership.WorkspaceID, membership.Role)
	if err != nil {
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", "server_error")}, nil
	}

	return &ssoRedirect{Location: h.frontendCallbackURL(tokens.AccessToken, tokens.RefreshToken, "")}, nil
}

func (h *SSOHandler) frontendCallbackURL(accessToken, refreshToken, errCode string) string {
	u, _ := url.Parse(fmt.Sprintf("%s/auth/sso/callback", h.frontendURL))
	q := u.Query()
	if errCode != "" {
		q.Set("error", errCode)
	}
	if accessToken != "" {
		q.Set("access_token", accessToken)
		q.Set("refresh_token", refreshToken)
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
	return &SSOSettingsOutput{Body: settings}, nil
}

type UpdateSSOSettingsInput struct {
	WorkspaceID uuid.UUID      `path:"ws_id"`
	Body        map[string]any
}

func (h *SSOHandler) UpdateSettings(ctx context.Context, input *UpdateSSOSettingsInput) (*SSOSettingsOutput, error) {
	ws, err := h.wsRepo.UpdateSSOSettings(ctx, input.WorkspaceID, input.Body)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("workspace not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update SSO settings")
	}
	return &SSOSettingsOutput{Body: ws.SSOSettings}, nil
}

// RegisterRoutes wires SSO endpoints onto the Huma API.
func (h *SSOHandler) RegisterRoutes(
	api huma.API,
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID:   "sso-authorize",
		Method:        http.MethodGet,
		Path:          "/api/auth/sso/{ws_slug}/authorize",
		Summary:       "Redirect to workspace SSO provider (OIDC)",
		Tags:          []string{"SSO"},
		DefaultStatus: http.StatusFound,
	}, h.Authorize)

	huma.Register(api, huma.Operation{
		OperationID:   "sso-callback",
		Method:        http.MethodGet,
		Path:          "/api/auth/sso/{ws_slug}/callback",
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

