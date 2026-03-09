package model

const (
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

// PaginationParams holds limit/offset values for paginated queries.
type PaginationParams struct {
	Limit  int
	Offset int
}

// Normalize clamps Limit to [1, MaxPageLimit] and Offset to >= 0.
func (p PaginationParams) Normalize() PaginationParams {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}
