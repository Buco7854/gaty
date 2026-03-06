package repository

import "github.com/Buco7854/gatie/internal/model"

// Sentinel errors — aliases to the canonical model-layer errors.
// Repository implementations return these; callers check model.ErrXxx.
var (
	ErrNotFound      = model.ErrNotFound
	ErrAlreadyExists = model.ErrAlreadyExists
	ErrUnauthorized  = model.ErrUnauthorized
)
