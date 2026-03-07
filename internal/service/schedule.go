package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/Buco7854/gatie/internal/repository"
	"github.com/google/uuid"
)

// ErrScheduleDenied is returned when a time-restriction schedule blocks access.
var ErrScheduleDenied = fmt.Errorf("access not allowed at this time")

type ScheduleService struct {
	schedules repository.AccessScheduleRepository
}

func NewScheduleService(schedules repository.AccessScheduleRepository) *ScheduleService {
	return &ScheduleService{schedules: schedules}
}

func (s *ScheduleService) Create(ctx context.Context, wsID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error) {
	if expr != nil {
		if err := validateExpr(expr, "expr"); err != nil {
			return nil, err
		}
	}
	return s.schedules.Create(ctx, wsID, nil, name, description, expr)
}

// CreateMember creates a personal schedule owned by a specific membership.
func (s *ScheduleService) CreateMember(ctx context.Context, wsID, membershipID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error) {
	if expr != nil {
		if err := validateExpr(expr, "expr"); err != nil {
			return nil, err
		}
	}
	return s.schedules.Create(ctx, wsID, &membershipID, name, description, expr)
}

func (s *ScheduleService) List(ctx context.Context, wsID uuid.UUID) ([]*model.AccessSchedule, error) {
	list, err := s.schedules.List(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*model.AccessSchedule{}
	}
	return list, nil
}

// ListMine returns personal schedules belonging to a specific membership.
func (s *ScheduleService) ListMine(ctx context.Context, wsID, membershipID uuid.UUID) ([]*model.AccessSchedule, error) {
	list, err := s.schedules.ListByMembership(ctx, membershipID, wsID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*model.AccessSchedule{}
	}
	return list, nil
}

func (s *ScheduleService) Get(ctx context.Context, scheduleID, wsID uuid.UUID) (*model.AccessSchedule, error) {
	return s.schedules.GetByID(ctx, scheduleID, wsID)
}

// GetPublic fetches a schedule by ID without workspace scoping (for internal access checks).
func (s *ScheduleService) GetPublic(ctx context.Context, scheduleID uuid.UUID) (*model.AccessSchedule, error) {
	return s.schedules.GetByIDPublic(ctx, scheduleID)
}

func (s *ScheduleService) Update(ctx context.Context, scheduleID, wsID uuid.UUID, name string, description *string, expr *model.ExprNode) (*model.AccessSchedule, error) {
	if expr != nil {
		if err := validateExpr(expr, "expr"); err != nil {
			return nil, err
		}
	}
	return s.schedules.Update(ctx, scheduleID, wsID, name, description, expr)
}

func (s *ScheduleService) Delete(ctx context.Context, scheduleID, wsID uuid.UUID) error {
	return s.schedules.Delete(ctx, scheduleID, wsID)
}

// Check returns nil if the schedule allows access at now, or ErrScheduleDenied.
// A nil Expr means always allowed. Use a NOT node to invert the expression.
func (s *ScheduleService) Check(schedule *model.AccessSchedule, now time.Time) error {
	if schedule.Expr == nil {
		return nil
	}
	if evalExpr(*schedule.Expr, now) {
		return nil
	}
	return fmt.Errorf("%w (schedule: %s)", ErrScheduleDenied, schedule.Name)
}

// evalExpr evaluates a boolean expression tree against the given time.
func evalExpr(node model.ExprNode, now time.Time) bool {
	switch node.Op {
	case "rule":
		if node.Rule == nil {
			return false
		}
		return scheduleRuleMatches(*node.Rule, now)
	case "not":
		if len(node.Children) != 1 {
			return false
		}
		return !evalExpr(node.Children[0], now)
	case "and":
		if len(node.Children) == 0 {
			return true
		}
		for _, c := range node.Children {
			if !evalExpr(c, now) {
				return false
			}
		}
		return true
	case "or":
		if len(node.Children) == 0 {
			return false
		}
		for _, c := range node.Children {
			if evalExpr(c, now) {
				return true
			}
		}
		return false
	}
	return false
}

// validateExpr recursively validates an expression tree.
func validateExpr(node *model.ExprNode, path string) error {
	switch node.Op {
	case "rule":
		if node.Rule == nil {
			return fmt.Errorf("%s: op=\"rule\" requires a rule object", path)
		}
		return validateRule(path, *node.Rule)
	case "not":
		if len(node.Children) != 1 {
			return fmt.Errorf("%s: op=\"not\" requires exactly 1 child, got %d", path, len(node.Children))
		}
		return validateExpr(&node.Children[0], path+".children[0]")
	case "and", "or":
		if len(node.Children) < 2 {
			return fmt.Errorf("%s: op=%q requires at least 2 children, got %d", path, node.Op, len(node.Children))
		}
		for i := range node.Children {
			if err := validateExpr(&node.Children[i], fmt.Sprintf("%s.children[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%s: unknown op %q (expected \"and\", \"or\", \"not\", \"rule\")", path, node.Op)
	}
}

func validateRule(path string, r model.ScheduleRule) error {
	switch r.Type {
	case "time_range":
		if r.StartTime != "" {
			if _, _, ok := parseHHMM(r.StartTime); !ok {
				return fmt.Errorf("%s: invalid start_time %q (expected HH:MM)", path, r.StartTime)
			}
		}
		if r.EndTime != "" {
			if _, _, ok := parseHHMM(r.EndTime); !ok {
				return fmt.Errorf("%s: invalid end_time %q (expected HH:MM)", path, r.EndTime)
			}
		}
		for _, d := range r.Days {
			if d < 0 || d > 6 {
				return fmt.Errorf("%s: invalid day %d (0=Sun..6=Sat)", path, d)
			}
		}
	case "weekdays_range":
		if r.StartDay == nil || r.EndDay == nil {
			return fmt.Errorf("%s: weekdays_range requires start_day and end_day", path)
		}
		if *r.StartDay < 0 || *r.StartDay > 6 || *r.EndDay < 0 || *r.EndDay > 6 {
			return fmt.Errorf("%s: day values must be 0–6", path)
		}
	case "date_range":
		if r.StartDate == "" || r.EndDate == "" {
			return fmt.Errorf("%s: date_range requires start_date and end_date", path)
		}
		if r.StartDate > r.EndDate {
			return fmt.Errorf("%s: start_date must be <= end_date", path)
		}
	case "day_of_month_range":
		if r.StartDOM == nil || r.EndDOM == nil {
			return fmt.Errorf("%s: day_of_month_range requires start_dom and end_dom", path)
		}
		if *r.StartDOM < 1 || *r.StartDOM > 31 || *r.EndDOM < 1 || *r.EndDOM > 31 {
			return fmt.Errorf("%s: start_dom and end_dom must be 1–31", path)
		}
	case "month_range":
		if r.StartMonth == nil || r.EndMonth == nil {
			return fmt.Errorf("%s: month_range requires start_month and end_month", path)
		}
		if *r.StartMonth < 1 || *r.StartMonth > 12 || *r.EndMonth < 1 || *r.EndMonth > 12 {
			return fmt.Errorf("%s: start_month and end_month must be 1–12", path)
		}
	default:
		return fmt.Errorf("%s: unknown rule type %q", path, r.Type)
	}
	return nil
}

func scheduleRuleMatches(r model.ScheduleRule, now time.Time) bool {
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
	return nowMins >= startH*60+startM && nowMins < endH*60+endM
}

func matchesWeekdaysRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartDay == nil || r.EndDay == nil {
		return false
	}
	dow := int(now.Weekday())
	start, end := *r.StartDay, *r.EndDay
	if start <= end {
		return dow >= start && dow <= end
	}
	return dow >= start || dow <= end
}

func matchesDateRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartDate == "" || r.EndDate == "" {
		return false
	}
	today := now.Format("2006-01-02")
	return today >= r.StartDate && today <= r.EndDate
}

func matchesDayOfMonthRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartDOM == nil || r.EndDOM == nil {
		return false
	}
	dom := now.Day()
	start, end := *r.StartDOM, *r.EndDOM
	if start <= end {
		return dom >= start && dom <= end
	}
	return dom >= start || dom <= end
}

func matchesMonthRange(r model.ScheduleRule, now time.Time) bool {
	if r.StartMonth == nil || r.EndMonth == nil {
		return false
	}
	month := int(now.Month())
	start, end := *r.StartMonth, *r.EndMonth
	if start <= end {
		return month >= start && month <= end
	}
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
