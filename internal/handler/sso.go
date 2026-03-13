package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
)

const (
	ssoCodePrefix = "sso:code:"
	ssoCodeTTL    = 60 * time.Second // one-time code valid for 60s
)

const ssoSecretMask = "***"

type SSOHandler struct {
	ssoSvc       *service.SSOService
	authSvc      *service.AuthService
	redis        *redis.Client
	frontendURL  string
	cookieSecure bool
}

func NewSSOHandler(
	ssoSvc *service.SSOService,
	authSvc *service.AuthService,
	redisClient *redis.Client,
	frontendURL string,
	cookieSecure bool,
) *SSOHandler {
	return &SSOHandler{ssoSvc: ssoSvc, authSvc: authSvc, redis: redisClient, frontendURL: frontendURL, cookieSecure: cookieSecure}
}

// ssoRedirect is the Huma output for HTTP redirect responses.
type ssoRedirect struct {
	Location string `header:"Location"`
}

// --- Public: List SSO providers ---

type SSOProvidersInput struct{}

type SSOProvidersOutput struct {
	Body []service.PublicSSOProvider
}

func (h *SSOHandler) ListProviders(ctx context.Context, input *SSOProvidersInput) (*SSOProvidersOutput, error) {
	providers, err := h.ssoSvc.ListPublicProviders()
	if errors.Is(err, service.ErrSSONotConfigured) {
		return &SSOProvidersOutput{Body: []service.PublicSSOProvider{}}, nil
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list SSO providers", err)
	}
	return &SSOProvidersOutput{Body: providers}, nil
}

// --- Authorize ---

type SSOAuthorizeInput struct {
	ProviderID string `path:"provider_id"`
	GateID     string `query:"gate_id"`
}

func (h *SSOHandler) Authorize(ctx context.Context, input *SSOAuthorizeInput) (*ssoRedirect, error) {
	authURL, err := h.ssoSvc.GenerateAuthURL(ctx, input.ProviderID, input.GateID)
	if errors.Is(err, service.ErrSSONotConfigured) {
		slog.Warn("sso authorize: not configured", "provider_id", input.ProviderID)
		return &ssoRedirect{Location: h.frontendCallbackURL("", input.GateID, "not_configured")}, nil
	}
	if err != nil {
		slog.Error("sso authorize: config error", "provider_id", input.ProviderID, "error", err)
		return &ssoRedirect{Location: h.frontendCallbackURL("", input.GateID, "config_error")}, nil
	}
	return &ssoRedirect{Location: authURL}, nil
}

// --- Callback ---

type SSOCallbackInput struct {
	ProviderID string `path:"provider_id"`
	Code       string `query:"code"`
	State      string `query:"state"`
	Error      string `query:"error"`
}

func (h *SSOHandler) Callback(ctx context.Context, input *SSOCallbackInput) (*ssoRedirect, error) {
	if input.Error != "" {
		gateID := h.ssoSvc.ConsumeState(ctx, input.State)
		slog.Warn("sso callback: provider returned error", "provider_id", input.ProviderID, "error", input.Error)
		return &ssoRedirect{Location: h.frontendCallbackURL("", gateID, input.Error)}, nil
	}

	member, gateID, err := h.ssoSvc.Callback(ctx, input.ProviderID, input.Code, input.State)
	if errors.Is(err, service.ErrSSOInvalidState) {
		slog.Warn("sso callback: invalid state", "provider_id", input.ProviderID)
		return &ssoRedirect{Location: h.frontendCallbackURL("", "", "invalid_state")}, nil
	}
	if errors.Is(err, service.ErrSSOAccessDenied) {
		slog.Warn("sso callback: access denied (auto-provision disabled)", "provider_id", input.ProviderID)
		return &ssoRedirect{Location: h.frontendCallbackURL("", gateID, "access_denied")}, nil
	}
	if err != nil {
		slog.Error("sso callback: error", "provider_id", input.ProviderID, "error", err)
		return &ssoRedirect{Location: h.frontendCallbackURL("", gateID, "server_error")}, nil
	}

	tokens, err := h.authSvc.IssueTokenPair(ctx, member.ID, member.Role)
	if err != nil {
		slog.Error("sso callback: failed to issue token pair", "member_id", member.ID, "error", err)
		return &ssoRedirect{Location: h.frontendCallbackURL("", gateID, "server_error")}, nil
	}

	code, err := h.storeSSOCode(ctx, tokens, member.ID.String(), gateID)
	if err != nil {
		slog.Error("sso callback: failed to store code", "error", err)
		return &ssoRedirect{Location: h.frontendCallbackURL("", gateID, "server_error")}, nil
	}

	return &ssoRedirect{Location: h.frontendCallbackURL(code, gateID, "")}, nil
}

// storeSSOCode stores the token pair in Redis behind a one-time opaque code.
func (h *SSOHandler) storeSSOCode(ctx context.Context, tokens *service.TokenPair, memberID, gateID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := hex.EncodeToString(b)
	payload, _ := json.Marshal(map[string]any{
		"access_token":     tokens.AccessToken,
		"refresh_token":    tokens.RefreshToken,
		"session_duration": tokens.SessionDuration.Seconds(),
		"member_id":        memberID,
		"gate_id":          gateID,
	})
	if err := h.redis.Set(ctx, ssoCodePrefix+code, string(payload), ssoCodeTTL).Err(); err != nil {
		return "", err
	}
	return code, nil
}

func (h *SSOHandler) frontendCallbackURL(code, gateID, errCode string) string {
	u, _ := url.Parse(fmt.Sprintf("%s/auth/sso/callback", h.frontendURL))
	q := u.Query()
	if errCode != "" {
		q.Set("error", errCode)
	}
	if code != "" {
		q.Set("code", code)
	}
	if gateID != "" {
		q.Set("gate_id", gateID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// --- SSO Code Exchange ---

type SSOExchangeInput struct {
	Body struct {
		Code string `json:"code" minLength:"1"`
	}
}

type SSOExchangeOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Type     string `json:"type"`
		MemberID string `json:"member_id"`
		GateID   string `json:"gate_id,omitempty"`
	}
}

// Exchange redeems a one-time SSO code, sets auth cookies, and returns session metadata.
func (h *SSOHandler) Exchange(ctx context.Context, input *SSOExchangeInput) (*SSOExchangeOutput, error) {
	val, err := h.redis.GetDel(ctx, ssoCodePrefix+input.Body.Code).Result()
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired code")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		return nil, huma.Error500InternalServerError("corrupt code payload")
	}

	var sessionDur time.Duration
	if sec, ok := payload["session_duration"].(float64); ok {
		sessionDur = time.Duration(sec) * time.Second
	}

	tokens := &service.TokenPair{
		AccessToken:     payload["access_token"].(string),
		RefreshToken:    payload["refresh_token"].(string),
		SessionDuration: sessionDur,
	}
	cookies := setAuthCookies(tokens, h.cookieSecure)
	out := &SSOExchangeOutput{}
	out.SetCookie = cookies[0]
	out.SetCookie2 = cookies[1]
	out.Body.Type = "local"
	if mid, ok := payload["member_id"].(string); ok {
		out.Body.MemberID = mid
	}
	if gid, ok := payload["gate_id"].(string); ok {
		out.Body.GateID = gid
	}
	return out, nil
}

// --- SSO Settings (admin, read-only from env) ---

type SSOSettingsOutput struct {
	Body []map[string]any
}

// GetSettings returns the SSO provider configurations with secrets masked.
func (h *SSOHandler) GetSettings(ctx context.Context, input *struct{}) (*SSOSettingsOutput, error) {
	providers := h.ssoSvc.Providers()
	masked := make([]map[string]any, len(providers))
	for i, p := range providers {
		// Marshal to map so we get all fields generically
		b, _ := json.Marshal(p)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		// Mask secret fields
		if secret, _ := m["client_secret"].(string); secret != "" {
			m["client_secret"] = ssoSecretMask
		}
		masked[i] = m
	}
	return &SSOSettingsOutput{Body: masked}, nil
}

// RegisterRoutes wires SSO endpoints onto the Huma API.
func (h *SSOHandler) RegisterRoutes(
	api huma.API,
	requireAdmin func(huma.Context, func(huma.Context)),
	ssoExchangeRateLimit func(huma.Context, func(huma.Context)),
) {
	huma.Register(api, huma.Operation{
		OperationID: "sso-providers",
		Method:      http.MethodGet,
		Path:        "/api/auth/sso/providers",
		Summary:     "List SSO providers (public)",
		Tags:        []string{"SSO"},
	}, h.ListProviders)

	huma.Register(api, huma.Operation{
		OperationID:   "sso-authorize",
		Method:        http.MethodGet,
		Path:          "/api/auth/sso/{provider_id}/authorize",
		Summary:       "Redirect to SSO provider",
		Tags:          []string{"SSO"},
		DefaultStatus: http.StatusFound,
	}, h.Authorize)

	huma.Register(api, huma.Operation{
		OperationID:   "sso-callback",
		Method:        http.MethodGet,
		Path:          "/api/auth/sso/{provider_id}/callback",
		Summary:       "SSO provider callback — stores tokens behind a one-time code and redirects to frontend",
		Tags:          []string{"SSO"},
		DefaultStatus: http.StatusFound,
	}, h.Callback)

	huma.Register(api, huma.Operation{
		OperationID: "sso-exchange",
		Method:      http.MethodPost,
		Path:        "/api/auth/sso/exchange",
		Summary:     "Exchange a one-time SSO code for tokens (code was received via redirect)",
		Tags:        []string{"SSO"},
		Middlewares: huma.Middlewares{ssoExchangeRateLimit},
	}, h.Exchange)

	huma.Register(api, huma.Operation{
		OperationID: "sso-settings-get",
		Method:      http.MethodGet,
		Path:        "/api/auth/sso/settings",
		Summary:     "Get SSO configuration (read-only, from env)",
		Tags:        []string{"SSO"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.GetSettings)
}
