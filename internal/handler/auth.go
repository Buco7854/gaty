package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
)

const (
	accessCookieName  = "gatie_access"
	refreshCookieName = "gatie_refresh"
)

// setAuthCookies builds Set-Cookie headers for both the access and refresh tokens.
func setAuthCookies(tokens *service.TokenPair, secure bool) [2]string {
	secureFlag := ""
	if secure {
		secureFlag = "; Secure"
	}
	accessMaxAge := int(tokens.AccessTokenTTL.Seconds())
	refreshMaxAge := int(tokens.SessionDuration.Seconds())
	if refreshMaxAge <= 0 {
		// "infinite" session: cap cookie at 90 days (browser max varies).
		refreshMaxAge = 90 * 24 * 60 * 60
	}
	return [2]string{
		fmt.Sprintf("%s=%s; HttpOnly%s; SameSite=Lax; Path=/api; Max-Age=%d",
			accessCookieName, tokens.AccessToken, secureFlag, accessMaxAge),
		fmt.Sprintf("%s=%s; HttpOnly%s; SameSite=Lax; Path=/api/auth; Max-Age=%d",
			refreshCookieName, tokens.RefreshToken, secureFlag, refreshMaxAge),
	}
}

// clearAuthCookies builds Set-Cookie headers that expire both cookies.
func clearAuthCookies(secure bool) [2]string {
	secureFlag := ""
	if secure {
		secureFlag = "; Secure"
	}
	return [2]string{
		fmt.Sprintf("%s=; HttpOnly%s; SameSite=Lax; Path=/api; Max-Age=0", accessCookieName, secureFlag),
		fmt.Sprintf("%s=; HttpOnly%s; SameSite=Lax; Path=/api/auth; Max-Age=0", refreshCookieName, secureFlag),
	}
}

type AuthHandler struct {
	authSvc      *service.AuthService
	members      *service.MemberService
	cookieSecure bool
}

func NewAuthHandler(authSvc *service.AuthService, members *service.MemberService, cookieSecure bool) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, members: members, cookieSecure: cookieSecure}
}

// --- Login ---

type LoginInput struct {
	Body struct {
		Username string `json:"username" minLength:"1" doc:"Member username"`
		Password string `json:"password" minLength:"1" doc:"Password"`
	}
}

type AuthOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Type   string       `json:"type"`
		Member model.Member `json:"member"`
	}
}

func (h *AuthHandler) Login(ctx context.Context, input *LoginInput) (*AuthOutput, error) {
	tokens, member, err := h.authSvc.Login(ctx, input.Body.Username, input.Body.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("login failed")
	}
	cookies := setAuthCookies(tokens, h.cookieSecure)
	resp := &AuthOutput{}
	resp.SetCookie = cookies[0]
	resp.SetCookie2 = cookies[1]
	resp.Body.Type = "member"
	resp.Body.Member = *member
	return resp, nil
}

// --- Refresh ---

type RefreshInput struct {
	RefreshCookie string `cookie:"gatie_refresh"`
}

type RefreshOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Type        string        `json:"type"`
		Member      *model.Member `json:"member,omitempty"`
		GateID      string        `json:"gate_id,omitempty"`
		Permissions []string      `json:"permissions,omitempty"`
	}
}

func (h *AuthHandler) Refresh(ctx context.Context, input *RefreshInput) (*RefreshOutput, error) {
	token := input.RefreshCookie
	if token == "" {
		return nil, huma.Error401Unauthorized("missing refresh token")
	}

	result, err := h.authSvc.Refresh(ctx, token)
	if errors.Is(err, service.ErrInvalidToken) {
		return nil, huma.Error401Unauthorized("invalid or expired refresh token")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("refresh failed")
	}

	cookies := setAuthCookies(result.Tokens, h.cookieSecure)
	resp := &RefreshOutput{}
	resp.SetCookie = cookies[0]
	resp.SetCookie2 = cookies[1]
	resp.Body.Type = result.Type

	switch result.Type {
	case "member":
		resp.Body.Member = result.Member
	case "pin_session":
		resp.Body.GateID = result.GateID.String()
		resp.Body.Permissions = result.Permissions
	}

	return resp, nil
}

// --- Logout ---

type LogoutInput struct {
	RefreshCookie string `cookie:"gatie_refresh"`
}

type LogoutOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
}

func (h *AuthHandler) Logout(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
	if input.RefreshCookie != "" {
		h.authSvc.RevokeRefreshToken(ctx, input.RefreshCookie)
	}
	cookies := clearAuthCookies(h.cookieSecure)
	return &LogoutOutput{SetCookie: cookies[0], SetCookie2: cookies[1]}, nil
}

// --- Me ---

type MeOutput struct {
	Body model.Member
}

func (h *AuthHandler) Me(ctx context.Context, _ *struct{}) (*MeOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	member, err := h.members.GetByID(ctx, memberID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to fetch member")
	}
	return &MeOutput{Body: *member}, nil
}

// RegisterRoutes wires auth endpoints onto the Huma API.
func (h *AuthHandler) RegisterRoutes(api huma.API, requireAuth func(huma.Context, func(huma.Context)), authRateLimit func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/api/auth/login",
		Summary:     "Login with username and password",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{authRateLimit},
	}, h.Login)

	huma.Register(api, huma.Operation{
		OperationID: "auth-refresh",
		Method:      http.MethodPost,
		Path:        "/api/auth/refresh",
		Summary:     "Refresh access token (reads refresh cookie)",
		Tags:        []string{"Auth"},
	}, h.Refresh)

	huma.Register(api, huma.Operation{
		OperationID: "auth-logout",
		Method:      http.MethodPost,
		Path:        "/api/auth/logout",
		Summary:     "Logout (clears session cookies and revokes refresh token)",
		Tags:        []string{"Auth"},
	}, h.Logout)

	huma.Register(api, huma.Operation{
		OperationID: "auth-me",
		Method:      http.MethodGet,
		Path:        "/api/auth/me",
		Summary:     "Get current authenticated member",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.Me)
}
