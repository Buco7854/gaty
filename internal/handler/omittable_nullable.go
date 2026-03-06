package handler

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/Buco7854/gatie/internal/model"
	"github.com/danielgtaylor/huma/v2"
)

// OmittableNullable is the HTTP transport representation of a field that can be omitted,
// set to null, or set to a value. Each state is tracked and can be checked in handling code.
//
//   - Sent=false            → field absent from request, leave unchanged
//   - Sent=true, Null=true  → field explicitly null, clear to NULL in DB
//   - Sent=true, Null=false → update to Value
//
// Use in handler input structs for PATCH endpoints:
//
//	OpenConfig OmittableNullable[model.ActionConfig] `json:"open_config,omitempty"`
//
// Convert to the domain layer type via ToModel() before passing to a service.
type OmittableNullable[T any] struct {
	Sent  bool
	Null  bool
	Value T
}

// ToModel converts the HTTP transport type to the domain patch type.
func (o OmittableNullable[T]) ToModel() model.OmittableNullable[T] {
	return model.OmittableNullable[T]{Sent: o.Sent, Null: o.Null, Value: o.Value}
}

// Schema implements huma.SchemaProvider so Huma generates correct OpenAPI docs.
func (o OmittableNullable[T]) Schema(r huma.Registry) *huma.Schema {
	return r.Schema(reflect.TypeOf(o.Value), true, "")
}

// UnmarshalJSON sets Sent=true whenever the field is present in the JSON payload,
// regardless of whether the value is null or a concrete value.
func (o *OmittableNullable[T]) UnmarshalJSON(b []byte) error {
	if len(b) > 0 {
		o.Sent = true
		if bytes.Equal(b, []byte("null")) {
			o.Null = true
			return nil
		}
		return json.Unmarshal(b, &o.Value)
	}
	return nil
}
