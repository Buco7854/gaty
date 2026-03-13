package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type MemberHandler struct {
	members *service.MemberService
}

func NewMemberHandler(members *service.MemberService) *MemberHandler {
	return &MemberHandler{members: members}
}

// --- Path params ---

type MemberIDPathParam struct {
	MemberID uuid.UUID `path:"member_id"`
}

// --- Response bodies ---

type MemberOutput struct {
	Body *model.Member
}

type ListMembersInput struct {
	PaginationQuery
}

type ListMembersOutput struct {
	Body PaginatedBody[*model.Member]
}

// --- Create ---

type CreateMemberInput struct {
	Body struct {
		Username    string     `json:"username" minLength:"1" maxLength:"50"`
		DisplayName *string    `json:"display_name,omitempty" maxLength:"100"`
		Password    string     `json:"password" minLength:"8"`
		Role        model.Role `json:"role,omitempty" default:"MEMBER"`
	}
}

func (h *MemberHandler) Create(ctx context.Context, input *CreateMemberInput) (*MemberOutput, error) {
	member, err := h.members.Create(ctx,
		input.Body.Username,
		input.Body.DisplayName,
		input.Body.Password,
		input.Body.Role,
	)
	if errors.Is(err, service.ErrUsernameTaken) {
		return nil, huma.Error409Conflict("username already taken")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create member", err)
	}
	return &MemberOutput{Body: member}, nil
}

// --- List ---

func (h *MemberHandler) List(ctx context.Context, input *ListMembersInput) (*ListMembersOutput, error) {
	p := input.Params()
	members, total, err := h.members.List(ctx, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list members", err)
	}
	return &ListMembersOutput{Body: NewPaginatedBody(members, total, p)}, nil
}

// --- Get ---

func (h *MemberHandler) Get(ctx context.Context, input *MemberIDPathParam) (*MemberOutput, error) {
	member, err := h.members.GetByID(ctx, input.MemberID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get member", err)
	}
	return &MemberOutput{Body: member}, nil
}

// --- Update ---

type UpdateMemberInput struct {
	MemberID uuid.UUID `path:"member_id"`
	Body     struct {
		DisplayName *string    `json:"display_name,omitempty" maxLength:"100"`
		Username    *string    `json:"username,omitempty" minLength:"1" maxLength:"50"`
		Role        *model.Role `json:"role,omitempty"`
	}
}

func (h *MemberHandler) Update(ctx context.Context, input *UpdateMemberInput) (*MemberOutput, error) {
	member, err := h.members.Update(ctx, input.MemberID,
		input.Body.DisplayName,
		input.Body.Username,
		input.Body.Role,
	)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if errors.Is(err, model.ErrAlreadyExists) {
		return nil, huma.Error409Conflict("username already taken")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update member", err)
	}
	return &MemberOutput{Body: member}, nil
}

// --- Delete ---

func (h *MemberHandler) Delete(ctx context.Context, input *MemberIDPathParam) (*struct{}, error) {
	err := h.members.Delete(ctx, input.MemberID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("member not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete member", err)
	}
	return nil, nil
}

// RegisterRoutes wires member endpoints onto the Huma API.
func (h *MemberHandler) RegisterRoutes(api huma.API, requireAdmin func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID:   "member-create",
		Method:        http.MethodPost,
		Path:          "/api/members",
		Summary:       "Create a member",
		Tags:          []string{"Members"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{requireAdmin},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID: "member-list",
		Method:      http.MethodGet,
		Path:        "/api/members",
		Summary:     "List all members",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "member-get",
		Method:      http.MethodGet,
		Path:        "/api/members/{member_id}",
		Summary:     "Get a member",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "member-update",
		Method:      http.MethodPatch,
		Path:        "/api/members/{member_id}",
		Summary:     "Update a member",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.Update)

	huma.Register(api, huma.Operation{
		OperationID: "member-delete",
		Method:      http.MethodDelete,
		Path:        "/api/members/{member_id}",
		Summary:     "Delete a member",
		Tags:        []string{"Members"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.Delete)
}
