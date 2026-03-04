package handler

import (
	"context"
	"net/http"

	"github.com/Buco7854/gaty/internal/repository"
	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
)

// SetupHandler handles the initial setup flow when no users exist yet.
type SetupHandler struct {
	users   *repository.UserRepository
	authSvc *service.AuthService
}

func NewSetupHandler(users *repository.UserRepository, authSvc *service.AuthService) *SetupHandler {
	return &SetupHandler{users: users, authSvc: authSvc}
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
		return nil, huma.Error500InternalServerError("failed to check setup status")
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
	Body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
}

func (h *SetupHandler) init(ctx context.Context, input *SetupInitInput) (*SetupInitOutput, error) {
	hasAny, err := h.users.HasAny(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check setup status")
	}
	if hasAny {
		return nil, huma.Error409Conflict("setup already completed")
	}

	tokens, _, err := h.authSvc.Register(ctx, input.Body.Email, input.Body.Password)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create admin user")
	}

	out := &SetupInitOutput{}
	out.Body.AccessToken = tokens.AccessToken
	out.Body.RefreshToken = tokens.RefreshToken
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
