package repository

import "encoding/json"

// Optional[T] distinguishes "field absent from JSON" (Set=false)
// from "field explicitly null" (Set=true, V=nil)
// and "field set to a value" (Set=true, V=&v).
//
// Use this in handler body structs and repository params for any PATCH field
// that is nullable in DB, so clients can explicitly clear a column to NULL
// rather than having "absent" and "null" collapse into the same meaning.
type Optional[T any] struct {
	Set bool
	V   *T
}

// UnmarshalJSON implements json.Unmarshaler.
// - Field absent in JSON  → UnmarshalJSON is never called, Set stays false.
// - Field is JSON null    → Set=true, V=nil.
// - Field has a value     → Set=true, V=&v.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.V = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.V = &v
	return nil
}
