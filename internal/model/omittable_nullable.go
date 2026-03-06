package model

// OmittableNullable represents a field that can be omitted, set to null, or set to a value.
//
//   - Sent=false              → field absent from request, leave unchanged
//   - Sent=true, Null=true   → field explicitly null, clear to NULL in DB
//   - Sent=true, Null=false  → update to Value
type OmittableNullable[T any] struct {
	Sent  bool
	Null  bool
	Value T
}
