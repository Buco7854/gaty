package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Buco7854/gatie/internal/middleware"
	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/Buco7854/gatie/internal/service"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type AccessScheduleHandler struct {
	schedules *service.ScheduleService
}

func NewAccessScheduleHandler(schedules *service.ScheduleService) *AccessScheduleHandler {
	return &AccessScheduleHandler{schedules: schedules}
}

type ScheduleOutput struct {
	Body *model.AccessSchedule
}

type ListSchedulesOutput struct {
	Body PaginatedBody[*model.AccessSchedule]
}

// --- Create (admin only) ---

type CreateScheduleInput struct {
	Body struct {
		Name        string          `json:"name" minLength:"1" maxLength:"100"`
		Description *string         `json:"description,omitempty" maxLength:"255"`
		Expr        *model.ExprNode `json:"expr,omitempty"`
	}
}

func (h *AccessScheduleHandler) Create(ctx context.Context, input *CreateScheduleInput) (*ScheduleOutput, error) {
	s, err := h.schedules.Create(ctx, input.Body.Name, input.Body.Description, input.Body.Expr)
	if err != nil {
		return nil, huma.Error400BadRequest("validation failed", &huma.ErrorDetail{Message: err.Error(), Location: "/body/expr"})
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- List (admin or gate manager) ---

type ListSchedulesInput struct {
	PaginationQuery
}

func (h *AccessScheduleHandler) List(ctx context.Context, input *ListSchedulesInput) (*ListSchedulesOutput, error) {
	p := input.Params()
	list, total, err := h.schedules.List(ctx, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list schedules", err)
	}
	return &ListSchedulesOutput{Body: NewPaginatedBody(list, total, p)}, nil
}

// --- Get ---

type SchedulePathParam struct {
	ScheduleID uuid.UUID `path:"schedule_id"`
}

func (h *AccessScheduleHandler) Get(ctx context.Context, input *SchedulePathParam) (*ScheduleOutput, error) {
	s, err := h.schedules.Get(ctx, input.ScheduleID)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule", err)
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Update (admin only) ---

type UpdateScheduleInput struct {
	ScheduleID uuid.UUID `path:"schedule_id"`
	Body       struct {
		Name        string          `json:"name" minLength:"1" maxLength:"100"`
		Description *string         `json:"description,omitempty" maxLength:"255"`
		Expr        *model.ExprNode `json:"expr,omitempty"`
	}
}

func (h *AccessScheduleHandler) Update(ctx context.Context, input *UpdateScheduleInput) (*ScheduleOutput, error) {
	s, err := h.schedules.Update(ctx, input.ScheduleID, input.Body.Name, input.Body.Description, input.Body.Expr)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error400BadRequest("validation failed", &huma.ErrorDetail{Message: err.Error(), Location: "/body/expr"})
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Delete (admin only) ---

func (h *AccessScheduleHandler) Delete(ctx context.Context, input *SchedulePathParam) (*struct{}, error) {
	err := h.schedules.Delete(ctx, input.ScheduleID)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete schedule", err)
	}
	return nil, nil
}

// --- Member personal schedules ---

// ListMine lists the current member's personal schedules.
func (h *AccessScheduleHandler) ListMine(ctx context.Context, input *ListSchedulesInput) (*ListSchedulesOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	p := input.Params()
	list, total, err := h.schedules.ListMine(ctx, memberID, p)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list personal schedules", err)
	}
	return &ListSchedulesOutput{Body: NewPaginatedBody(list, total, p)}, nil
}

// CreateMine creates a personal schedule for the current member.
func (h *AccessScheduleHandler) CreateMine(ctx context.Context, input *CreateScheduleInput) (*ScheduleOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	s, err := h.schedules.CreateMember(ctx, memberID, input.Body.Name, input.Body.Description, input.Body.Expr)
	if err != nil {
		return nil, huma.Error400BadRequest("validation failed", &huma.ErrorDetail{Message: err.Error(), Location: "/body/expr"})
	}
	return &ScheduleOutput{Body: s}, nil
}

// UpdateMine updates a personal schedule — only if it belongs to the current member.
func (h *AccessScheduleHandler) UpdateMine(ctx context.Context, input *UpdateScheduleInput) (*ScheduleOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	// Fetch to verify ownership before updating.
	existing, err := h.schedules.Get(ctx, input.ScheduleID)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule", err)
	}
	if existing.MemberID == nil || *existing.MemberID != memberID {
		return nil, huma.Error403Forbidden("you can only edit your own schedules")
	}
	s, err := h.schedules.Update(ctx, input.ScheduleID, input.Body.Name, input.Body.Description, input.Body.Expr)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error400BadRequest("validation failed", &huma.ErrorDetail{Message: err.Error(), Location: "/body/expr"})
	}
	return &ScheduleOutput{Body: s}, nil
}

// DeleteMine deletes a personal schedule — only if it belongs to the current member.
func (h *AccessScheduleHandler) DeleteMine(ctx context.Context, input *SchedulePathParam) (*struct{}, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	existing, err := h.schedules.Get(ctx, input.ScheduleID)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule", err)
	}
	if existing.MemberID == nil || *existing.MemberID != memberID {
		return nil, huma.Error403Forbidden("you can only delete your own schedules")
	}
	if err := h.schedules.Delete(ctx, input.ScheduleID); err != nil {
		return nil, huma.Error500InternalServerError("failed to delete schedule", err)
	}
	return nil, nil
}

// GetMine retrieves a specific personal schedule — only if it belongs to the current member.
func (h *AccessScheduleHandler) GetMine(ctx context.Context, input *SchedulePathParam) (*ScheduleOutput, error) {
	memberID, ok := middleware.MemberIDFromContext(ctx)
	if !ok {
		return nil, huma.Error401Unauthorized("unauthorized")
	}
	s, err := h.schedules.Get(ctx, input.ScheduleID)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule", err)
	}
	if s.MemberID == nil || *s.MemberID != memberID {
		return nil, huma.Error404NotFound("schedule not found")
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Routes ---

func (h *AccessScheduleHandler) RegisterRoutes(api huma.API, requireAuth, requireAdmin, adminOrGateManager func(huma.Context, func(huma.Context))) {
	// Schedules (admin-managed)
	huma.Register(api, huma.Operation{
		OperationID:   "schedule-create",
		Method:        http.MethodPost,
		Path:          "/api/schedules",
		Summary:       "Create a time-restriction schedule",
		Tags:          []string{"Schedules"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{requireAdmin},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-list",
		Method:      http.MethodGet,
		Path:        "/api/schedules",
		Summary:     "List time-restriction schedules",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{adminOrGateManager},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-get",
		Method:      http.MethodGet,
		Path:        "/api/schedules/{schedule_id}",
		Summary:     "Get a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{adminOrGateManager},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-update",
		Method:      http.MethodPut,
		Path:        "/api/schedules/{schedule_id}",
		Summary:     "Replace a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.Update)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-delete",
		Method:      http.MethodDelete,
		Path:        "/api/schedules/{schedule_id}",
		Summary:     "Delete a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{requireAdmin},
	}, h.Delete)

	// Member personal schedules (any authenticated member manages their own)
	huma.Register(api, huma.Operation{
		OperationID:   "my-schedule-create",
		Method:        http.MethodPost,
		Path:          "/api/members/me/schedules",
		Summary:       "Create a personal time-restriction schedule",
		Tags:          []string{"Schedules"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{requireAuth},
	}, h.CreateMine)

	huma.Register(api, huma.Operation{
		OperationID: "my-schedule-list",
		Method:      http.MethodGet,
		Path:        "/api/members/me/schedules",
		Summary:     "List my personal time-restriction schedules",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.ListMine)

	huma.Register(api, huma.Operation{
		OperationID: "my-schedule-get",
		Method:      http.MethodGet,
		Path:        "/api/members/me/schedules/{schedule_id}",
		Summary:     "Get a personal time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.GetMine)

	huma.Register(api, huma.Operation{
		OperationID: "my-schedule-update",
		Method:      http.MethodPut,
		Path:        "/api/members/me/schedules/{schedule_id}",
		Summary:     "Replace a personal time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.UpdateMine)

	huma.Register(api, huma.Operation{
		OperationID: "my-schedule-delete",
		Method:      http.MethodDelete,
		Path:        "/api/members/me/schedules/{schedule_id}",
		Summary:     "Delete a personal time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{requireAuth},
	}, h.DeleteMine)
}
