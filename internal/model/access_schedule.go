package model

import (
	"time"

	"github.com/google/uuid"
)

// ScheduleRule describes a single time restriction rule.
// A schedule passes if the current time satisfies ANY rule (OR logic).
//
// Rule types:
//
//	time_range:     allowed during a time window, optionally filtered to specific days of week.
//	                { "type":"time_range", "days":[1,2,3,4,5], "start_time":"10:00", "end_time":"12:00" }
//	                days = [0..6] (0=Sun). Omit or leave empty for all days.
//
//	weekdays_range: allowed on a contiguous range of weekdays (wraps around Sunday).
//	                { "type":"weekdays_range", "start_day":6, "end_day":0 }
//	                e.g., start_day=6 (Sat), end_day=0 (Sun) → week-end.
//
//	date_range:     allowed between two calendar dates (inclusive).
//	                { "type":"date_range", "start_date":"2026-01-01", "end_date":"2026-12-31" }
type ScheduleRule struct {
	Type      string `json:"type"`
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
}

// AccessSchedule is a named, reusable set of time restriction rules attached to a workspace.
// It can be referenced by gate access codes (PINs) or member-gate policies.
type AccessSchedule struct {
	ID          uuid.UUID      `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Rules       []ScheduleRule `json:"rules"`
	CreatedAt   time.Time      `json:"created_at"`
}
