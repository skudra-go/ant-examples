// Package services holds the business logic. It knows about models and ant, and
// nothing about HTTP, so a command or a test can call it directly.
package services

import "errors"

// Errors a caller can act on. Routes turn these into status codes.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	ErrInvalid  = errors.New("invalid")
)
