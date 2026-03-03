package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/Buco7854/gaty/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type MemberHandler struct {
	members *service.MemberService
}

func NewMemberHandler(members *service.MemberService) *MemberHandler {
	return &MemberHandler{members: members}
}

// --- Member login ---

type MemberLoginInput struct {
	Body struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
		Login       string    `json:"login" minLength:"1"`
		Password    string    `json:"password" minLength:"1"`
	}
}

type MemberLoginOutput struct {
	Body struct {
		AccessToken string `json:"access_token"`
	}
}

func (h *MemberHandler) Login(ctx context.Context, input *MemberLoginInput) (*MemberLoginOutput, error) {
	token, _, err := h.members.Login(ctx, input.Body.WorkspaceID, input.Body.Login, input.Body.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("login failed")
	}
	out := &MemberLoginOutput{}
	out.Body.AccessToken = token
	return out, nil
}

// --- Create member ---

type MemberWorkspacePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
}

type MemberIDPathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	MemberID    uuid.UUID `path:"member_id"`
}

type CreateMemberInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		DisplayName string  `json:"display_name" minLength:"1" maxLength:"100"`
		Username    string  `json:"username" minLength:"1" maxLength:"50"`
		Password    string  `json:"password" minLength:"8"`
		Email       *string `json:"email,omitempty" format:"email"`
	}
}

type MemberOutput struct {
	Body *memberBody
}

type memberBody struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	DisplayName string     `json:"display_name"`
	Email       *string    `json:"email,omitempty"`
	Username    string     `json:"username"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
}

func (h *MemberHandler) Create(ctx context.Context, input *CreateMemberInput) (*MemberOutput, error) {
	member, err := h.members.Create(ctx,
		input.WorkspaceID,
		input.Body.DisplayName,
		input.Body.Email,
		input.Body.Username,
		input.Body.Password,
	)
	if errors.Is(err, service.ErrUsernameTaken) {
		return nil, huma.Error409Conflict("username already taken in this workspace")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create member")
	}
	return &MemberOutput{Body: toMemberBody(member)}, nil
}

// --- List members ---

type ListMembersOutput struct {
	Body []*memberBody
}

func (h *MemberHandler) List(ctx context.Context, input *MemberWorkspacePathParam) (*ListMembersOutput, error) {
	members, err := h.members.List(ctx, input.WorkspaceID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list members")
	}
	bodies := make([]*memberBody, len(members))
	for i, m := range members {
		bodies[i] = toMemberBody(m)
	}
	return &ListMembersOutput{Body: bodies}, nil
}

// --- Get member ---

func (h *MemberHandler) Get(ctx context.Context, input *MemberIDPathParam) (*MemberOutput, error) {
	member, err := h.members.GetByID(ctx, input.MemberID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get member")
	}
	return &MemberOutput{Body: toMemberBody(member)}, nil
}

// --- Update member ---

type UpdateMemberInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	MemberID    uuid.UUID `path:"member_id"`
	Body        struct {
		DisplayName string  `json:"display_name" minLength:"1" maxLength:"100"`
		Email       *string `json:"email,omitempty" format:"email"`
	}
}

func (h *MemberHandler) Update(ctx context.Context, input *UpdateMemberInput) (*MemberOutput, error) {
	member, err := h.members.Update(ctx, input.MemberID, input.WorkspaceID, input.Body.DisplayName, input.Body.Email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update member")
	}
	return &MemberOutput{Body: toMemberBody(member)}, nil
}

// --- Delete member ---

func (h *MemberHandler) Delete(ctx context.Context, input *MemberIDPathParam) (*struct{}, error) {
	err := h.members.Delete(ctx, input.MemberID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete member")
	}
	return nil, nil
}

// RegisterRoutes wires member endpoints onto the Huma API.
func (h *MemberHandler) RegisterRoutes(
	api huma.API,
	wsAdmin func(huma.Context, func(huma.Context)),
) {
	// Public: member login (no workspace auth required, workspace_id is in body)
	huma.Register(api, huma.Operation{
		OperationID: "member-login",
		Method:      http.MethodPost,
		Path:        "/api/auth/member/login",
		Summary:     "Member login (username or email)",
		Tags:        []string{"Members"},
	}, h.Login)

	huma.Register(api, huma.Operation{
		OperationID:   "member-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/members",
		Summary:       "Create a managed member",
		Tags:          []string{"Members"},
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID: "member-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members",
		Summary:     "List managed members",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "member-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/members/{member_id}",
		Summary:     "Get a managed member",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "member-update",
		Method:      http.MethodPatch,
		Path:        "/api/workspaces/{ws_id}/members/{member_id}",
		Summary:     "Update a managed member",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Update)

	huma.Register(api, huma.Operation{
		OperationID: "member-delete",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/members/{member_id}",
		Summary:     "Delete a managed member",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Delete)
}

func toMemberBody(m *model.Member) *memberBody {
	return &memberBody{
		ID:          m.ID,
		WorkspaceID: m.WorkspaceID,
		DisplayName: m.DisplayName,
		Email:       m.Email,
		Username:    m.Username,
		UserID:      m.UserID,
	}
}
