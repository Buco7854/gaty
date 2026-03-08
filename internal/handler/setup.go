package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
)

// SetupHandler handles the initial setup flow when no users exist yet.
type SetupHandler struct {
	users        repository.UserRepository
	authSvc      *service.AuthService
	cookieSecure bool
}

func NewSetupHandler(users repository.UserRepository, authSvc *service.AuthService, cookieSecure bool) *SetupHandler {
	return &SetupHandler{users: users, authSvc: authSvc, cookieSecure: cookieSecure}
}

// --- Status ---

type SetupStatusOutput struct {
	Body struct {
		SetupRequired bool `json:"setup_required"`
	}
}

func (h *SetupHandler) status(ctx context.Context, _ *struct{}) (*SetupStatusOutput, error) {
	hasAny, err := h.users.HasAny(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check setup status", err)
	}
	out := &SetupStatusOutput{}
	out.Body.SetupRequired = !hasAny
	return out, nil
}

// --- Init ---

type SetupInitInput struct {
	Body struct {
		Email    string `json:"email" format:"email"`
		Password string `json:"password" minLength:"8"`
	}
}

type SetupInitOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		User model.User `json:"user"`
	}
}

func (h *SetupHandler) init(ctx context.Context, input *SetupInitInput) (*SetupInitOutput, error) {
	hasAny, err := h.users.HasAny(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check setup status", err)
	}
	if hasAny {
		return nil, huma.Error409Conflict("setup already completed")
	}

	tokens, user, err := h.authSvc.Register(ctx, input.Body.Email, input.Body.Password)
	if errors.Is(err, service.ErrWeakPassword) {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create admin user", err)
	}

	cookies := setAuthCookies(tokens, h.cookieSecure)
	out := &SetupInitOutput{}
	out.SetCookie = cookies[0]
	out.SetCookie2 = cookies[1]
	out.Body.User = *user
	return out, nil
}

// RegisterRoutes wires setup endpoints — no auth required.
func (h *SetupHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "setup-status",
		Method:      http.MethodGet,
		Path:        "/api/setup/status",
		Summary:     "Check if initial setup is required",
		Tags:        []string{"Setup"},
	}, h.status)

	huma.Register(api, huma.Operation{
		OperationID: "setup-init",
		Method:      http.MethodPost,
		Path:        "/api/setup/init",
		Summary:     "Create the first admin user (only when no users exist)",
		Tags:        []string{"Setup"},
	}, h.init)
}
