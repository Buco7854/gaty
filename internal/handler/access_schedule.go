package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Buco7854/gaty/internal/model"
	"github.com/Buco7854/gaty/internal/repository"
	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

type AccessScheduleHandler struct {
	schedules *repository.AccessScheduleRepository
}

func NewAccessScheduleHandler(schedules *repository.AccessScheduleRepository) *AccessScheduleHandler {
	return &AccessScheduleHandler{schedules: schedules}
}

// --- Response types ---

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
		Name        string                `json:"name" minLength:"1" maxLength:"100"`
		Description *string               `json:"description,omitempty" maxLength:"255"`
		Rules       []model.ScheduleRule  `json:"rules"`
	}
}

func (h *AccessScheduleHandler) Create(ctx context.Context, input *CreateScheduleInput) (*ScheduleOutput, error) {
	if err := validateScheduleRules(input.Body.Rules); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	s, err := h.schedules.Create(ctx, input.WorkspaceID, input.Body.Name, input.Body.Description, input.Body.Rules)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create schedule")
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- List ---

func (h *AccessScheduleHandler) List(ctx context.Context, input *WorkspacePathParam) (*ListSchedulesOutput, error) {
	list, err := h.schedules.List(ctx, input.WorkspaceID)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to list schedules")
	}
	if list == nil {
		list = []*model.AccessSchedule{}
	}
	return &ListSchedulesOutput{Body: list}, nil
}

// --- Get ---

type SchedulePathParam struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	ScheduleID  uuid.UUID `path:"schedule_id"`
}

func (h *AccessScheduleHandler) Get(ctx context.Context, input *SchedulePathParam) (*ScheduleOutput, error) {
	s, err := h.schedules.GetByID(ctx, input.ScheduleID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to get schedule")
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Update ---

type UpdateScheduleInput struct {
	WorkspaceID uuid.UUID `path:"ws_id"`
	ScheduleID  uuid.UUID `path:"schedule_id"`
	Body        struct {
		Name        string                `json:"name" minLength:"1" maxLength:"100"`
		Description *string               `json:"description,omitempty" maxLength:"255"`
		Rules       []model.ScheduleRule  `json:"rules"`
	}
}

func (h *AccessScheduleHandler) Update(ctx context.Context, input *UpdateScheduleInput) (*ScheduleOutput, error) {
	if err := validateScheduleRules(input.Body.Rules); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	s, err := h.schedules.Update(ctx, input.ScheduleID, input.WorkspaceID, input.Body.Name, input.Body.Description, input.Body.Rules)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to update schedule")
	}
	return &ScheduleOutput{Body: s}, nil
}

// --- Delete ---

func (h *AccessScheduleHandler) Delete(ctx context.Context, input *SchedulePathParam) (*struct{}, error) {
	err := h.schedules.Delete(ctx, input.ScheduleID, input.WorkspaceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error404NotFound("schedule not found")
	}
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to delete schedule")
	}
	return nil, nil
}

// RegisterRoutes wires access schedule endpoints onto the Huma API.
func (h *AccessScheduleHandler) RegisterRoutes(api huma.API, wsAdmin func(huma.Context, func(huma.Context))) {
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
		Middlewares: huma.Middlewares{wsAdmin},
	}, h.List)

	huma.Register(api, huma.Operation{
		OperationID: "schedule-get",
		Method:      http.MethodGet,
		Path:        "/api/workspaces/{ws_id}/schedules/{schedule_id}",
		Summary:     "Get a time-restriction schedule",
		Tags:        []string{"Schedules"},
		Middlewares: huma.Middlewares{wsAdmin},
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

// --- Schedule evaluation logic ---

// CheckSchedule returns nil if the schedule allows access at the given time,
// or a non-nil error describing why access is denied.
// An empty rules slice means always allowed.
func CheckSchedule(s *model.AccessSchedule, now time.Time) error {
	if len(s.Rules) == 0 {
		return nil
	}
	for _, rule := range s.Rules {
		if ruleMatches(rule, now) {
			return nil
		}
	}
	return fmt.Errorf("access not allowed at this time (schedule: %s)", s.Name)
}

func ruleMatches(r model.ScheduleRule, now time.Time) bool {
	switch r.Type {
	case "time_range":
		return matchesTimeRange(r, now)
	case "weekdays_range":
		return matchesWeekdaysRange(r, now)
	case "date_range":
		return matchesDateRange(r, now)
	case "day_of_month_range":
		return matchesDayOfMonthRange(r, now)
	case "month_range":
		return matchesMonthRange(r, now)
	}
	return false
}

// matchesTimeRange: current day must be in r.Days (if set) AND current time in [start_time, end_time).
func matchesTimeRange(r model.ScheduleRule, now time.Time) bool {
	if len(r.Days) > 0 {
		dow := int(now.Weekday())
		inDays := false
		for _, d := range r.Days {
			if d == dow {
				inDays = true
				break
			}
		}
		if !inDays {
			return false
		}
	}
	if r.StartTime == "" || r.EndTime == "" {
		return true
	}
	startH, startM, ok1 := parseHHMM(r.StartTime)
	endH, endM, ok2 := parseHHMM(r.EndTime)
	if !ok1 || !ok2 {
		return false
	}
	nowMins := now.Hour()*60 + now.Minute()
	startMins := startH*60 + startM
	endMins := endH*60 + endM
	return nowMins >= startMins && nowMins < endMins
}

// matchesWeekdaysRange: current weekday is in the inclusive range [start_day, end_day].
// Wraps around (e.g., Sat=6 → Sun=0 covers the week-end).
func matchesWeekdaysRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartDay == nil || r.EndDay == nil {
		return false
	}
	dow := int(now.Weekday())
	start, end := *r.StartDay, *r.EndDay
	if start <= end {
		return dow >= start && dow <= end
	}
	// Wraps around: e.g., Fri(5) → Mon(1) means Fri, Sat, Sun, Mon
	return dow >= start || dow <= end
}

// matchesDateRange: current date is in [start_date, end_date] inclusive.
func matchesDateRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartDate == "" || r.EndDate == "" {
		return false
	}
	today := now.Format("2006-01-02")
	return today >= r.StartDate && today <= r.EndDate
}

// matchesDayOfMonthRange: current day of month is within [start_dom, end_dom] (recurring, wraps).
func matchesDayOfMonthRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartDOM == nil || r.EndDOM == nil {
		return false
	}
	dom := now.Day()
	start, end := *r.StartDOM, *r.EndDOM
	if start <= end {
		return dom >= start && dom <= end
	}
	// Wraps around month boundary (e.g., 28–5 covers end of month + start of next).
	return dom >= start || dom <= end
}

// matchesMonthRange: current month is within [start_month, end_month] (recurring, wraps).
func matchesMonthRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartMonth == nil || r.EndMonth == nil {
		return false
	}
	month := int(now.Month())
	start, end := *r.StartMonth, *r.EndMonth
	if start <= end {
		return month >= start && month <= end
	}
	// Wraps around year boundary (e.g., 11–2 covers Nov, Dec, Jan, Feb).
	return month >= start || month <= end
}

func parseHHMM(s string) (int, int, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// validateScheduleRules returns an error if any rule is malformed.
func validateScheduleRules(rules []model.ScheduleRule) error {
	for i, r := range rules {
		switch r.Type {
		case "time_range":
			if r.StartTime != "" {
				if _, _, ok := parseHHMM(r.StartTime); !ok {
					return fmt.Errorf("rule[%d]: invalid start_time %q (expected HH:MM)", i, r.StartTime)
				}
			}
			if r.EndTime != "" {
				if _, _, ok := parseHHMM(r.EndTime); !ok {
					return fmt.Errorf("rule[%d]: invalid end_time %q (expected HH:MM)", i, r.EndTime)
				}
			}
			for _, d := range r.Days {
				if d < 0 || d > 6 {
					return fmt.Errorf("rule[%d]: invalid day %d (0=Sun..6=Sat)", i, d)
				}
			}
		case "weekdays_range":
			if r.StartDay == nil || r.EndDay == nil {
				return fmt.Errorf("rule[%d]: weekdays_range requires start_day and end_day", i)
			}
			if *r.StartDay < 0 || *r.StartDay > 6 || *r.EndDay < 0 || *r.EndDay > 6 {
				return fmt.Errorf("rule[%d]: day values must be 0–6", i)
			}
		case "date_range":
			if r.StartDate == "" || r.EndDate == "" {
				return fmt.Errorf("rule[%d]: date_range requires start_date and end_date", i)
			}
			if r.StartDate > r.EndDate {
				return fmt.Errorf("rule[%d]: start_date must be <= end_date", i)
			}
		case "day_of_month_range":
			if r.StartDOM == nil || r.EndDOM == nil {
				return fmt.Errorf("rule[%d]: day_of_month_range requires start_dom and end_dom", i)
			}
			if *r.StartDOM < 1 || *r.StartDOM > 31 || *r.EndDOM < 1 || *r.EndDOM > 31 {
				return fmt.Errorf("rule[%d]: start_dom and end_dom must be 1–31", i)
			}
		case "month_range":
			if r.StartMonth == nil || r.EndMonth == nil {
				return fmt.Errorf("rule[%d]: month_range requires start_month and end_month", i)
			}
			if *r.StartMonth < 1 || *r.StartMonth > 12 || *r.EndMonth < 1 || *r.EndMonth > 12 {
				return fmt.Errorf("rule[%d]: start_month and end_month must be 1–12", i)
			}
		default:
			return fmt.Errorf("rule[%d]: unknown rule type %q", i, r.Type)
		}
	}
	return nil
}
