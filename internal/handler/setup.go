package handler

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
)

// SetupHandler handles the initial setup flow when no members exist yet.
type SetupHandler struct {
	members      *service.MemberService
	authSvc      *service.AuthService
	cookieSecure bool
	initMu       sync.Mutex // serializes init to prevent race conditions
}

func NewSetupHandler(members *service.MemberService, authSvc *service.AuthService, cookieSecure bool) *SetupHandler {
	return &SetupHandler{members: members, authSvc: authSvc, cookieSecure: cookieSecure}
}

// --- Status ---

type SetupStatusOutput struct {
	Body struct {
		SetupRequired bool `json:"setup_required"`
	}
}

func (h *SetupHandler) status(ctx context.Context, _ *struct{}) (*SetupStatusOutput, error) {
	hasAny, err := h.members.HasAny(ctx)
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
		Username    string  `json:"username"`
		Password    string  `json:"password" minLength:"8"`
		DisplayName *string `json:"display_name,omitempty"`
	}
}

type SetupInitOutput struct {
	SetCookie  string `header:"Set-Cookie"`
	SetCookie2 string `header:"Set-Cookie2"`
	Body       struct {
		Member model.Member `json:"member"`
	}
}

func (h *SetupHandler) init(ctx context.Context, input *SetupInitInput) (*SetupInitOutput, error) {
	// Serialize init requests to prevent race conditions where two concurrent
	// requests both pass the HasAny check and create two admin members.
	h.initMu.Lock()
	defer h.initMu.Unlock()

	hasAny, err := h.members.HasAny(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to check setup status", err)
	}
	if hasAny {
		return nil, huma.Error409Conflict("setup already completed")
	}

	tokens, member, err := h.authSvc.Register(ctx, input.Body.Username, input.Body.Password, input.Body.DisplayName)
	if errors.Is(err, service.ErrWeakPassword) {
		return nil, huma.Error400BadRequest(err.Error())
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create admin member", err)
	}

	cookies := setAuthCookies(tokens, h.cookieSecure)
	out := &SetupInitOutput{}
	out.SetCookie = cookies[0]
	out.SetCookie2 = cookies[1]
	out.Body.Member = *member
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
		Summary:     "Create the first admin member (only when no members exist)",
		Tags:        []string{"Setup"},
	}, h.init)
}
