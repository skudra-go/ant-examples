// Package services holds the business logic. It knows about models and ant, and
// nothing about HTTP.
package services

import (
	"errors"

	"github.com/skudra-go/ant"
)

// Errors a caller can act on. Routes turn these into status codes.
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalid           = errors.New("invalid")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrConflict          = errors.New("changed by someone else")
)

func translate(err error) error {
	switch {
	case errors.Is(err, ant.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, ant.ErrConcurrency):
		return ErrConflict
	}

	return err
}
