package model

import (
	"time"

	"github.com/google/uuid"
)

// ScheduleRule describes a single time restriction rule.
//
// Rule types:
//
//	time_range:          allowed during a time window, optionally filtered to specific days of week.
//	                     { "type":"time_range", "days":[1,2,3,4,5], "start_time":"10:00", "end_time":"12:00" }
//	                     days = [0..6] (0=Sun). Omit or leave empty for all days.
//
//	weekdays_range:      allowed on a contiguous range of weekdays (wraps around Sunday).
//	                     { "type":"weekdays_range", "start_day":6, "end_day":0 }
//	                     e.g., start_day=6 (Sat), end_day=0 (Sun) → week-end.
//
//	date_range:          allowed between two calendar dates (inclusive, non-recurring).
//	                     { "type":"date_range", "start_date":"2026-01-01", "end_date":"2026-12-31" }
//
//	day_of_month_range:  allowed on a recurring day-of-month window (1–31).
//	                     { "type":"day_of_month_range", "start_dom":1, "end_dom":7 }
//	                     e.g., days 1–7 → first week of every month.
//	                     Wraps when start_dom > end_dom (e.g., 28–5 spans a month boundary).
//
//	month_range:         allowed during a recurring month window (1=Jan … 12=Dec).
//	                     { "type":"month_range", "start_month":1, "end_month":3 }
//	                     e.g., Jan–Mar every year. Wraps when start_month > end_month.
type ScheduleRule struct {
	Type string `json:"type"`
	// time_range fields
	Days      []int  `json:"days,omitempty"`       // [0..6], 0=Sun; empty = all days
	StartTime string `json:"start_time,omitempty"` // "HH:MM"
	EndTime   string `json:"end_time,omitempty"`   // "HH:MM"
	// weekdays_range fields
	StartDay *int `json:"start_day,omitempty"` // 0..6
	EndDay   *int `json:"end_day,omitempty"`   // 0..6
	// date_range fields
	StartDate string `json:"start_date,omitempty"` // "YYYY-MM-DD"
	EndDate   string `json:"end_date,omitempty"`   // "YYYY-MM-DD"
	// day_of_month_range fields
	StartDOM *int `json:"start_dom,omitempty"` // 1..31
	EndDOM   *int `json:"end_dom,omitempty"`   // 1..31
	// month_range fields
	StartMonth *int `json:"start_month,omitempty"` // 1..12
	EndMonth   *int `json:"end_month,omitempty"`   // 1..12
}

// ExprNode is a node in a boolean expression tree used to define schedule conditions.
//
// Op values:
//
//	"and"  — all children must match (logical AND)
//	"or"   — at least one child must match (logical OR)
//	"not"  — inverts its single child (logical NOT)
//	"rule" — leaf node: evaluates the embedded ScheduleRule
//
// Examples:
//
//	weekdays only:  { "op":"rule", "rule":{"type":"weekdays_range","start_day":1,"end_day":5} }
//	business hours: { "op":"and", "children":[weekdays, time_window] }
//	except lunch:   { "op":"and", "children":[business_hours, {"op":"not","children":[lunch]}] }
type ExprNode struct {
	Op       string        `json:"op"`                 // "and" | "or" | "not" | "rule"
	Children []ExprNode    `json:"children,omitempty"` // for op = "and" | "or" | "not"
	Rule     *ScheduleRule `json:"rule,omitempty"`     // for op = "rule" only
}

// AccessSchedule is a named, reusable boolean expression attached to a workspace.
// It can be referenced by gate access codes (PINs) or member-gate policies.
//
// Expr is the root of the expression tree. nil means always allowed.
// Access is granted when Expr evaluates to true. Use a NOT node to invert.
//
// MembershipID IS NULL  → workspace schedule (admin-managed, assignable to PINs/members).
// MembershipID IS NOT NULL → member personal schedule (only usable by that member on their own tokens).
type AccessSchedule struct {
	ID           uuid.UUID  `json:"id"`
	WorkspaceID  uuid.UUID  `json:"workspace_id"`
	MembershipID *uuid.UUID `json:"membership_id,omitempty"`
	Name         string     `json:"name"`
	Description  *string    `json:"description,omitempty"`
	Expr         *ExprNode  `json:"expr"` // nil = no restriction (always allowed)
	CreatedAt    time.Time  `json:"created_at"`
}
