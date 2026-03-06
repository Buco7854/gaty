package repository

import "github.com/Buco7854/gatie/internal/model"

// OmittableNullable is a type alias for model.OmittableNullable.
// Use model.OmittableNullable[T] in new code; this alias keeps the postgres layer unchanged.
type OmittableNullable[T any] = model.OmittableNullable[T]
