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
	Body []*model.AccessSchedule
}

// --- Create ---

type CreateScheduleInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	Body        struct {
		Name        string          `json:"name" minLength:"1" maxLength:"100"`
		Description *string         `json:"description,omitempty" maxLength:"255"`
		Expr        *model.ExprNode `json:"expr,omitempty"`
	}
}

func (h *AccessScheduleHandler) Create(ctx context.Context, input *CreateScheduleInput) (*ScheduleOutput, error) {
	s, err := h.schedules.Create(ctx, input.WorkspaceID, input.Body.Name, input.Body.Description, input.Body.Expr)
	if err != nil {
		return nil, huma.Error400BadRequest("validation failed", &huma.ErrorDetail{Message: err.Error(), Location: "/body/expr"})
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- List ---

func (h *AccessScheduleHandler) List(ctx context.Context, input *WorkspacePathParam) (*ListSchedulesOutput, error) {
	list, err := h.schedules.List(ctx, input.WorkspaceID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list schedules", err)
	}
	return &ListSchedulesOutput{Body: list}, nil
}

// --- Get ---

type SchedulePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	ScheduleID  uuid.UUID `path:"schedule_id"`
}

func (h *AccessScheduleHandler) Get(ctx context.Context, input *SchedulePathParam) (*ScheduleOutput, error) {
	s, err := h.schedules.Get(ctx, input.ScheduleID, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule", err)
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Update ---

type UpdateScheduleInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	ScheduleID  uuid.UUID `path:"schedule_id"`
	Body        struct {
		Name        string          `json:"name" minLength:"1" maxLength:"100"`
		Description *string         `json:"description,omitempty" maxLength:"255"`
		Expr        *model.ExprNode `json:"expr,omitempty"`
	}
}

func (h *AccessScheduleHandler) Update(ctx context.Context, input *UpdateScheduleInput) (*ScheduleOutput, error) {
	s, err := h.schedules.Update(ctx, input.ScheduleID, input.WorkspaceID, input.Body.Name, input.Body.Description, input.Body.Expr)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error400BadRequest("validation failed", &huma.ErrorDetail{Message: err.Error(), Location: "/body/expr"})
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Delete ---

func (h *AccessScheduleHandler) Delete(ctx context.Context, input *SchedulePathParam) (*struct{}, error) {
	err := h.schedules.Delete(ctx, input.ScheduleID, input.WorkspaceID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete schedule", err)
	}
	return nil, nil
}

// --- Routes ---

func (h *AccessScheduleHandler) RegisterRoutes(api huma.API, wsMember func(huma.Context, func(huma.Context)), wsAdmin func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{
		OperationID:   "schedule-create",
		Method:        http.MethodPost,
		Path:          "/api/workspaces/{ws_id}/schedules",
		Summary:       "Create a time-restriction schedule",
		Tags:          []string{"Schedules"},
		DefaultStatus: http.StatusCreated,
		Middlewares:   huma.Middlewares{wsAdmin},
	}, h.Create)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-list",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/schedules",
		Summary:     "List all time-restriction schedules in a workspace",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/schedules/{schedule_id}",
		Summary:     "Get a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{wsMember},
	}, h.Get)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-update",
		Method:      http.MethodPut,
		Path:        "/api/workspaces/{ws_id}/schedules/{schedule_id}",
		Summary:     "Replace a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Update)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-delete",
		Method:      http.MethodDelete,
		Path:        "/api/workspaces/{ws_id}/schedules/{schedule_id}",
		Summary:     "Delete a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.Delete)
}
