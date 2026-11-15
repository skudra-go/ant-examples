// Package core is the data layer, shaped like listmonk's internal/core: one
// method per operation, used by the HTTP handlers. A Core is bound to one
// database handle: the root, a per-request session, or a transaction.
package core

import (
	"errors"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var (
	// ErrNotFound is returned when the row asked for does not exist.
	ErrNotFound = errors.New("not found")

	// ErrExists is returned when a unique value is taken.
	ErrExists = errors.New("already exists")

	// ErrInvalid is returned for input the core refuses.
	ErrInvalid = errors.New("invalid input")
)

// Core holds the database handle every method uses.
type Core struct {
	db *models.AppDB
}

// New returns a core on db.
func New(db *models.AppDB) *Core {
	return &Core{db: db}
}

// With returns a core on another handle of the same database, such as a
// session for one request or the tx inside db.Tx, so that several core
// methods can share one transaction.
func (c *Core) With(db *models.AppDB) *Core {
	return &Core{db: db}
}

// DB returns the handle this core uses.
func (c *Core) DB() *models.AppDB {
	return c.db
}

// notFound maps ant's not-found error to the core's.
func notFound(err error) error {
	if errors.Is(err, ant.ErrNotFound) {
		return ErrNotFound
	}

	return err
}

// Sorting is a whitelist of columns a request may sort by. A request names the
// column with a string, and only a name in the map reaches the query.
type Sorting map[string]func(desc bool) ant.Sort

// SortBy returns the ordering term for a column, in either direction.
func SortBy[V any](field *V) func(desc bool) ant.Sort {
	return func(desc bool) ant.Sort {
		if desc {
			return ant.Col(field).Desc()
		}

		return ant.Col(field).Asc()
	}
}

// Term returns the ordering for name and order ("asc" or "desc"), or the
// fallback when the name is not in the whitelist.
func (s Sorting) Term(name, order, fallback string) ant.Sort {
	term, ok := s[name]
	if !ok {
		term = s[fallback]
	}

	return term(order != "asc")
}
