package services

import (
	"context"
	"iter"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/audit/models"
)

// Filter narrows a search. Empty fields do not apply.
type Filter struct {
	Service, Action, Outcome, Actor string
	Person                          string
	Role                            string
	Text                            string
	From, To                        time.Time
	Meta                            map[string]any
}

// Search reads events.
type Search struct {
	db *models.AppDB
}

// NewSearch returns the service.
func NewSearch(db *models.AppDB) *Search { return &Search{db} }

// query builds the filter into a query. A time range is the most valuable part:
// PostgreSQL skips whole partitions that cannot hold a match.
func (s *Search) query(f Filter) *ant.Query[models.Event] {
	type q = *ant.Query[models.Event]

	return s.db.Events.Query().
		When(f.Service != "", func(q q) q {
			return q.Where(ant.Eq(&models.Events.Service, f.Service))
		}).
		When(f.Action != "", func(q q) q {
			return q.Where(ant.Like(&models.Events.Action, f.Action))
		}).
		When(f.Outcome != "", func(q q) q {
			return q.Where(ant.Eq(&models.Events.Outcome, f.Outcome))
		}).
		When(f.Actor != "", func(q q) q {
			return q.Where(ant.Eq(&models.Events.ActorID, f.Actor))
		}).
		When(!f.From.IsZero(), func(q q) q {
			return q.Where(ant.Gte(&models.Events.OccurredAt, f.From))
		}).
		When(!f.To.IsZero(), func(q q) q {
			return q.Where(ant.Lt(&models.Events.OccurredAt, f.To))
		}).
		When(f.Person != "", func(q q) q {
			conds := []ant.Cond{ant.Eq(&models.Persons.Ref, f.Person)}

			if f.Role != "" {
				conds = append(conds, ant.Eq(&models.Persons.Role, f.Role))
			}

			return q.Where(ant.Has(&models.Events.Persons, conds...))
		}).
		When(f.Text != "", func(q q) q {
			// The generated column and its GIN index, not a scan of the summary.
			return q.Where(
				ant.SQLCond(
					"? @@ websearch_to_tsquery('simple', ?)",
					ant.Col(&models.Events.Search),
					f.Text,
				),
			)
		}).
		When(len(f.Meta) > 0, func(q q) q {
			return q.Where(postgres.JSONContains(&models.Events.Metadata, f.Meta))
		})
}

// Events returns a page of matching events, newest first. A cursor, not an offset,
// so that page 10,000 costs what page 1 does, and rows arriving meanwhile do not
// shift it. Persons load in one more query, however many events there are.
func (s *Search) Events(
	ctx context.Context,
	f Filter,
	cursor string,
	size int,
) (*ant.CursorPage[models.Event], error) {
	return s.query(f).
		Order(ant.Col(&models.Events.OccurredAt).Desc(), ant.Col(&models.Events.ID).Desc()).
		With(&models.Events.PersonCount, ant.CountRelated(&models.Events.Persons)).
		Include(
			&models.Events.Persons,
			func(q *ant.Query[models.Person]) *ant.Query[models.Person] {
				return q.OrderBy(&models.Persons.Ref)
			},
		).
		After(ctx, cursor, size)
}

// Event returns one event and its persons.
func (s *Search) Event(ctx context.Context, id string) (*models.Event, error) {
	event, err := s.db.Events.Where(ant.Eq(&models.Events.ID, id)).
		With(&models.Events.PersonCount, ant.CountRelated(&models.Events.Persons)).
		Include(&models.Events.Persons).
		First(ctx)

	return event, translate(err)
}

// Chain returns an event and everything that followed from it, through ParentID,
// however deep, with one recursive query.
func (s *Search) Chain(ctx context.Context, id string) ([]*models.Event, error) {
	return s.db.Events.Where(ant.Eq(&models.Events.ID, id)).
		Recursive(&models.Events.Children).
		OrderBy(&models.Events.OccurredAt).
		All(ctx)
}

// Export streams matching events without holding them in memory. Each yields a
// row at a time as it comes from the database, which is what a large export needs.
func (s *Search) Export(ctx context.Context, f Filter) iter.Seq2[models.Event, error] {
	return s.query(f).OrderBy(&models.Events.OccurredAt).Each(ctx)
}
