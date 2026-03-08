package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
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
	accessMaxAge := int(service.AccessTokenTTL.Seconds())
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
	users        repository.UserRepository
	cookieSecure bool
}

func NewAuthHandler(authSvc *service.AuthService, users repository.UserRepository, cookieSecure bool) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, users: users, cookieSecure: cookieSecure}
}

// --- Register ---

type RegisterInput struct {
	Body struct {
		Email    string `json:"email" format:"email" doc:"User email address"`
		Password string `json:"password" minLength:"8" maxLength:"128" doc:"Password (min 8 chars, must contain uppercase, lowercase, and digit)"`
	}
}

type GlobalAuthOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Type string     `json:"type"`
		User model.User `json:"user"`
	}
}

func (h *AuthHandler) Register(ctx context.Context, input *RegisterInput) (*GlobalAuthOutput, error) {
	tokens, user, err := h.authSvc.Register(ctx, input.Body.Email, input.Body.Password)
	if errors.Is(err, service.ErrWeakPassword) {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if errors.Is(err, service.ErrEmailTaken) {
		return nil, huma.Error409Conflict("registration failed")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("registration failed")
	}
	cookies := setAuthCookies(tokens, h.cookieSecure)
	resp := &GlobalAuthOutput{}
	resp.SetCookie = cookies[0]
	resp.SetCookie2 = cookies[1]
	resp.Body.Type = "global"
	resp.Body.User = *user
	return resp, nil
}

// --- Login (global) ---

type LoginInput struct {
	Body struct {
		Email    string `json:"email" format:"email"`
		Password string `json:"password"`
	}
}

func (h *AuthHandler) Login(ctx context.Context, input *LoginInput) (*GlobalAuthOutput, error) {
	tokens, user, err := h.authSvc.Login(ctx, input.Body.Email, input.Body.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("login failed")
	}
	cookies := setAuthCookies(tokens, h.cookieSecure)
	resp := &GlobalAuthOutput{}
	resp.SetCookie = cookies[0]
	resp.SetCookie2 = cookies[1]
	resp.Body.Type = "global"
	resp.Body.User = *user
	return resp, nil
}

// --- Login (local membership) ---

type LoginLocalInput struct {
	Body struct {
		WorkspaceID  uuid.UUID `json:"workspace_id"`
		LocalUsername string    `json:"local_username" minLength:"1"`
		Password     string    `json:"password" minLength:"1"`
	}
}

type LocalAuthOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Type         string              `json:"type"`
		MembershipID string              `json:"membership_id"`
		WorkspaceID  string              `json:"workspace_id"`
		Role         model.WorkspaceRole `json:"role"`
		DisplayName  string              `json:"display_name,omitempty"`
	}
}

func (h *AuthHandler) LoginLocal(ctx context.Context, input *LoginLocalInput) (*LocalAuthOutput, error) {
	tokens, membership, err := h.authSvc.LoginLocal(ctx, input.Body.WorkspaceID, input.Body.LocalUsername, input.Body.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("login failed")
	}
	cookies := setAuthCookies(tokens, h.cookieSecure)
	out := &LocalAuthOutput{}
	out.SetCookie = cookies[0]
	out.SetCookie2 = cookies[1]
	out.Body.Type = "local"
	out.Body.MembershipID = membership.ID.String()
	out.Body.WorkspaceID = membership.WorkspaceID.String()
	out.Body.Role = membership.Role
	if membership.DisplayName != nil {
		out.Body.DisplayName = *membership.DisplayName
	}
	return out, nil
}

// --- Refresh ---

type RefreshInput struct {
	RefreshCookie string `cookie:"gatie_refresh"`
}

type RefreshOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Type         string              `json:"type"`
		User         *model.User         `json:"user,omitempty"`
		MembershipID string              `json:"membership_id,omitempty"`
		WorkspaceID  string              `json:"workspace_id,omitempty"`
		Role         model.WorkspaceRole `json:"role,omitempty"`
		DisplayName  string              `json:"display_name,omitempty"`
		GateID       string              `json:"gate_id,omitempty"`
		Permissions  []string            `json:"permissions,omitempty"`
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
	case "global":
		resp.Body.User = result.User
	case "local":
		if result.Membership != nil {
			resp.Body.MembershipID = result.Membership.ID.String()
			resp.Body.WorkspaceID = result.Membership.WorkspaceID.String()
			resp.Body.Role = result.Membership.Role
			if result.Membership.DisplayName != nil {
				resp.Body.DisplayName = *result.Membership.DisplayName
			}
		}
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

// --- Merge (link local membership to global user) ---

type MergeInput struct {
	Body struct {
		WorkspaceID  uuid.UUID `json:"workspace_id"`
		LocalUsername string    `json:"local_username" minLength:"1"`
		Password     string    `json:"password" minLength:"1"`
	}
}

func (h *AuthHandler) Merge(ctx context.Context, input *MergeInput) (*struct{}, error) {
	userID, _ := middleware.UserIDFromContext(ctx)
	err := h.authSvc.Merge(ctx, userID, input.Body.WorkspaceID, input.Body.LocalUsername, input.Body.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if errors.Is(err, service.ErrAlreadyMerged) {
		return nil, huma.Error409Conflict("membership already linked to a user account")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("merge failed")
	}
	return nil, nil
}

// --- Me ---

type MeOutput struct {
	Body model.User
}

func (h *AuthHandler) Me(ctx context.Context, _ *struct{}) (*MeOutput, error) {
	userID, _ := middleware.UserIDFromContext(ctx)
	user, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to fetch user")
	}
	return &MeOutput{Body: *user}, nil
}

// RegisterRoutes wires auth endpoints onto the Huma API.
func (h *AuthHandler) RegisterRoutes(api huma.API, requireAuth func(huma.Context, func(huma.Context)), authRateLimit func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-register",
		Method:      http.MethodPost,
		Path:        "/api/auth/register",
		Summary:     "Register a new platform user",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{authRateLimit},
	}, h.Register)

	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/api/auth/login",
		Summary:     "Login with email and password (platform user)",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{authRateLimit},
	}, h.Login)

	huma.Register(api, huma.Operation{
		OperationID: "auth-login-local",
		Method:      http.MethodPost,
		Path:        "/api/auth/login/local",
		Summary:     "Login as a managed member (local credentials)",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{authRateLimit},
	}, h.LoginLocal)

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
		OperationID: "auth-merge",
		Method:      http.MethodPost,
		Path:        "/api/auth/merge",
		Summary:     "Link a local membership to the authenticated platform user",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.Merge)

	huma.Register(api, huma.Operation{
		OperationID: "auth-me",
		Method:      http.MethodGet,
		Path:        "/api/auth/me",
		Summary:     "Get current platform user",
		Tags:        []string{"Auth"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.Me)
}
