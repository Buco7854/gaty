package handler

import "github.com/Buco7854/gatie/internal/model"

// PaginationQuery is a Huma mixin for list endpoints.
// Embed it in your input struct to get ?limit=&offset= query parameters.
type PaginationQuery struct {
	Limit  int `query:"limit" required:"false" minimum:"1" maximum:"100" default:"50" doc:"Max items to return (1–100, default 50)"`
	Offset int `query:"offset" required:"false" minimum:"0" default:"0" doc:"Number of items to skip"`
}

// Params returns normalized PaginationParams.
func (q PaginationQuery) Params() model.PaginationParams {
	return model.PaginationParams{Limit: q.Limit, Offset: q.Offset}.Normalize()
}

// PaginatedBody wraps a paginated list response with metadata.
type PaginatedBody[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NewPaginatedBody creates a PaginatedBody from a slice and total count.
func NewPaginatedBody[T any](items []T, total int, p model.PaginationParams) PaginatedBody[T] {
	if items == nil {
		items = []T{}
	}
	return PaginatedBody[T]{
		Items:  items,
		Total:  total,
		Limit:  p.Limit,
		Offset: p.Offset,
	}
}
