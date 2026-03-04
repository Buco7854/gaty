package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Buco7854/gaty/internal/middleware"
	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authSvc *service.AuthService
	users   *repository.UserRepository
}

func NewAuthHandler(authSvc *service.AuthService, users *repository.UserRepository) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, users: users}
}

// --- Register ---

type RegisterInput struct {
	Body struct {
		Email    string `json:"email" format:"email" doc:"User email address"`
		Password string `json:"password" minLength:"8" doc:"Password (min 8 chars)"`
	}
}

type AuthOutput struct {
	Body struct {
		AccessToken  string     `json:"access_token"`
		RefreshToken string     `json:"refresh_token"`
		User         model.User `json:"user"`
	}
}

func (h *AuthHandler) Register(ctx context.Context, input *RegisterInput) (*AuthOutput, error) {
	tokens, user, err := h.authSvc.Register(ctx, input.Body.Email, input.Body.Password)
	if errors.Is(err, service.ErrEmailTaken) {
		return nil, huma.Error409Conflict("email already taken")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("registration failed")
	}
	resp := &AuthOutput{}
	resp.Body.AccessToken = tokens.AccessToken
	resp.Body.RefreshToken = tokens.RefreshToken
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

func (h *AuthHandler) Login(ctx context.Context, input *LoginInput) (*AuthOutput, error) {
	tokens, user, err := h.authSvc.Login(ctx, input.Body.Email, input.Body.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if err != nil {
		slog.Error("login failed", "error", err)
		return nil, huma.Error500InternalServerError("login failed")
	}
	resp := &AuthOutput{}
	resp.Body.AccessToken = tokens.AccessToken
	resp.Body.RefreshToken = tokens.RefreshToken
	resp.Body.User = *user
	return resp, nil
}

// --- Login (local membership) ---

type LoginLocalInput struct {
	Body struct {
		WorkspaceID   uuid.UUID `json:"workspace_id"`
		LocalUsername string    `json:"local_username" minLength:"1"`
		Password      string    `json:"password" minLength:"1"`
	}
}

type LocalAuthOutput struct {
	Body struct {
		AccessToken  string                    `json:"access_token"`
		RefreshToken string                    `json:"refresh_token"`
		Membership   *model.WorkspaceMembership `json:"membership"`
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
	out := &LocalAuthOutput{}
	out.Body.AccessToken = tokens.AccessToken
	out.Body.RefreshToken = tokens.RefreshToken
	out.Body.Membership = membership
	return out, nil
}

// --- Refresh ---

type RefreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token"`
	}
}

type RefreshOutput struct {
	Body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
}

func (h *AuthHandler) Refresh(ctx context.Context, input *RefreshInput) (*RefreshOutput, error) {
	tokens, err := h.authSvc.Refresh(ctx, input.Body.RefreshToken)
	if errors.Is(err, service.ErrInvalidToken) {
		return nil, huma.Error401Unauthorized("invalid or expired refresh token")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("refresh failed")
	}
	resp := &RefreshOutput{}
	resp.Body.AccessToken = tokens.AccessToken
	resp.Body.RefreshToken = tokens.RefreshToken
	return resp, nil
}

// --- Merge (link local membership to global user) ---

type MergeInput struct {
	Body struct {
		WorkspaceID   uuid.UUID `json:"workspace_id"`
		LocalUsername string    `json:"local_username" minLength:"1"`
		Password      string    `json:"password" minLength:"1"`
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
func (h *AuthHandler) RegisterRoutes(api huma.API, requireAuth func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-register",
		Method:      http.MethodPost,
		Path:        "/api/auth/register",
		Summary:     "Register a new platform user",
		Tags:        []string{"Auth"},
	}, h.Register)

	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/api/auth/login",
		Summary:     "Login with email and password (platform user)",
		Tags:        []string{"Auth"},
	}, h.Login)

	huma.Register(api, huma.Operation{
		OperationID: "auth-login-local",
		Method:      http.MethodPost,
		Path:        "/api/auth/login/local",
		Summary:     "Login as a managed member (local credentials)",
		Tags:        []string{"Auth"},
	}, h.LoginLocal)

	huma.Register(api, huma.Operation{
		OperationID: "auth-refresh",
		Method:      http.MethodPost,
		Path:        "/api/auth/refresh",
		Summary:     "Refresh access token",
		Tags:        []string{"Auth"},
	}, h.Refresh)

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
