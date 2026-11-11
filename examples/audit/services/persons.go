package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/audit/models"
)

func translate(err error) error {
	if errors.Is(err, ant.ErrNotFound) {
		return ErrNotFound
	}

	return err
}

// Persons answers questions about one person.
type Persons struct {
	db *models.AppDB
}

// NewPersons returns the service.
func NewPersons(db *models.AppDB) *Persons { return &Persons{db} }

// Step is one entry in a person's timeline.
type Step struct {
	EventID  string              `json:"eventId"`
	At       time.Time           `json:"at"`
	Service  string              `json:"service"`
	Action   string              `json:"action"`
	Actor    string              `json:"actor"`
	Role     string              `json:"role"`
	Number   int64               `json:"number"`
	Previous sql.Null[time.Time] `json:"-"`
	// GapHours is the time since the step before.
	GapHours *float64 `json:"gapHours,omitempty"`
}

// Steps is the probe for Step.
var Steps = ant.Fields[Step]()

// Timeline returns what happened to a person, oldest first. The join names both
// key columns, so PostgreSQL pairs each month with its own month instead of searching
// every partition, and the window functions run over the joined rows in the
// database: the step number, and the time of the step before.
func (s *Persons) Timeline(ctx context.Context, ref string, limit int) ([]Step, error) {
	byTime := ant.OrderedBy(ant.Col(&models.Persons.OccurredAt).Asc())

	rows := s.db.Persons.Where(ant.Eq(&models.Persons.Ref, ref)).
		OrderBy(&models.Persons.OccurredAt).
		Limit(limit)
	joined := rows.JoinOn[models.Event](
		ant.EqCol(&models.Persons.EventID, &models.Events.ID),
		ant.EqCol(&models.Persons.OccurredAt, &models.Events.OccurredAt))

	steps, err := ant.Project[Step](joined,
		ant.Into(&Steps.EventID, &models.Events.ID),
		ant.Into(&Steps.At, &models.Persons.OccurredAt),
		ant.Into(&Steps.Service, &models.Events.Service),
		ant.Into(&Steps.Action, &models.Events.Action),
		ant.Into(&Steps.Actor, &models.Events.ActorName),
		ant.Into(&Steps.Role, &models.Persons.Role),
		ant.IntoExpr(&Steps.Number, ant.RowNumber().Over(byTime)),
		ant.IntoExpr(&Steps.Previous, ant.Lag(&models.Persons.OccurredAt).Over(byTime)),
	).All(ctx)
	if err != nil {
		return nil, err
	}

	for i := range steps {
		if steps[i].Previous.Valid {
			hours := steps[i].At.Sub(steps[i].Previous.V).Hours()
			steps[i].GapHours = &hours
		}
	}

	return steps, nil
}

// RoleCount is how often a person appeared in one role.
type RoleCount struct {
	Role   string `json:"role"`
	Events int64  `json:"events"`
	Actors int64  `json:"actors"`
}

// RoleCounts is the probe for RoleCount.
var RoleCounts = ant.Fields[RoleCount]()

// Summary is everything known about a person in aggregate.
type Summary struct {
	Overview *models.PersonOverview `json:"overview"`
	Roles    []RoleCount            `json:"roles"`
}

// Summary reads the overview from a view, and adds a breakdown by role.
func (s *Persons) Summary(ctx context.Context, ref string) (*Summary, error) {
	overview, err := s.db.PersonOverviews.Where(ant.Eq(&models.PersonOverviews.Ref, ref)).First(ctx)
	if err != nil {
		return nil, translate(err)
	}

	roles, err := ant.Project[RoleCount](
		s.db.Persons.Where(ant.Eq(&models.Persons.Ref, ref)).
			JoinOn[models.Event](ant.EqCol(&models.Persons.EventID, &models.Events.ID)).
			GroupBy(&models.Persons.Role),
		ant.Into(&RoleCounts.Role, &models.Persons.Role),
		ant.IntoExpr(&RoleCounts.Events, ant.Count()),
		ant.IntoExpr(&RoleCounts.Actors, ant.CountDistinct(&models.Events.ActorID)),
	).All(ctx)
	if err != nil {
		return nil, err
	}

	return &Summary{Overview: overview, Roles: roles}, nil
}

// Erasure says what erasing a person changed.
type Erasure struct {
	Rows    int64  `json:"rows"`
	Pseudo  string `json:"pseudonym"`
	EventID string `json:"eventId"`
}

// Erase removes a person's identity from every month, as a data protection request
// requires, and keeps the events: the record that something happened stays, without
// saying who to. It is one transaction, and records itself as an event, so the
// erasure can be audited. The pseudonym lets the same person be counted once.
func (s *Persons) Erase(ctx context.Context, ref, requestedBy string) (*Erasure, error) {
	sum := sha256.Sum256([]byte(ref))
	pseudonym := "erased:" + hex.EncodeToString(sum[:8])
	out := &Erasure{Pseudo: pseudonym}
	err := s.db.Tx(ctx, func(tx *models.AppDB) error {
		updated, err := tx.Persons.Where(ant.Eq(&models.Persons.Ref, ref)).UpdateReturning(ctx,
			ant.Set(&models.Persons.Ref, pseudonym),
			ant.Set(&models.Persons.Name, nil),
			ant.Set(&models.Persons.Fields, postgres.Array[string]{}),
		)
		if err != nil {
			return err
		}

		if len(updated) == 0 {
			return ErrNotFound
		}

		out.Rows = int64(len(updated))

		record := toEvent(EventInput{
			OccurredAt: time.Now(),
			Service:    "audit",
			Action:     "person.erased",
			ActorID:    requestedBy,
			ActorName:  requestedBy,
			Summary:    fmt.Sprintf("Erased %d rows", out.Rows),
			Metadata:   map[string]any{"pseudonym": pseudonym},
		})

		out.EventID = record.ID

		if err := tx.Events.Insert(ctx, record); err != nil {
			return err
		}

		return updateTotals(ctx, tx, []*models.Event{record})
	})

	return out, err
}
