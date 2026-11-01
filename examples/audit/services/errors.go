// Package services holds the business logic. It knows about models and ant, and
// nothing about HTTP.
package services

import "errors"

// Errors a caller can act on. Routes turn these into status codes.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
)
