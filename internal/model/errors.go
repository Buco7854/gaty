package model

import "errors"

// Sentinel domain errors. These are the canonical errors for the application.
// Repository implementations wrap these; service and handler layers check them.
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrUnauthorized  = errors.New("unauthorized")
)
