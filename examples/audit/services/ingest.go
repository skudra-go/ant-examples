package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"

	"github.com/skudra-go/ant-examples/audit/models"
)

// PersonInput is someone an event touched, as a sender describes them.
type PersonInput struct {
	Ref    string
	Name   *string
	Role   string
	Fields []string
}

// EventInput is an event as a sender describes it. Persons arrive inside it, and
// are stored in a table of their own.
type EventInput struct {
	ID           string
	OccurredAt   time.Time
	Service      string
	Action       string
	Outcome      string
	ActorID      string
	ActorName    string
	ResourceType string
	ResourceID   string
	IP           *string
	ParentID     *string
	Summary      string
	Metadata     map[string]any
	Persons      []PersonInput
}

// Ingest stores what senders report.
type Ingest struct {
	db *models.AppDB
}

// NewIngest returns the service.
func NewIngest(db *models.AppDB) *Ingest { return &Ingest{db} }

// Result says what a batch did.
type Result struct {
	Stored     int      `json:"stored"`
	Duplicates int      `json:"duplicates"`
	IDs        []string `json:"ids"`
}

// toEvent splits an input into the rows it becomes. Each person repeats the
// event's time, which places it in the same month's partition.
func toEvent(input EventInput) *models.Event {
	event := &models.Event{
		ID:           input.ID,
		OccurredAt:   input.OccurredAt.UTC().Truncate(time.Microsecond),
		Service:      input.Service,
		Action:       input.Action,
		Outcome:      input.Outcome,
		ActorID:      input.ActorID,
		ActorName:    input.ActorName,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		IP:           input.IP,
		ParentID:     input.ParentID,
		Summary:      input.Summary,
		Metadata:     ant.JSON[map[string]any]{V: input.Metadata},
	}

	if event.ID == "" {
		event.ID = ids.UUID()
	}

	if event.Outcome == "" {
		event.Outcome = models.OutcomeSuccess
	}

	if event.Metadata.V == nil {
		event.Metadata.V = map[string]any{}
	}

	for _, person := range input.Persons {
		event.Persons = append(event.Persons, models.Person{
			OccurredAt: event.OccurredAt,
			Ref:        person.Ref,
			Name:       person.Name,
			Role:       person.Role,
			Fields:     person.Fields,
		})
	}

	return event
}

// Store writes events, their persons, and the running totals in one transaction.
// A sender that retries after a timeout sends the same IDs again, so an ID already
// stored is counted as a duplicate and skipped, which makes the call safe to repeat.
//
// Insert walks each event's Persons, and writes each table in batches, so a
// thousand events cost a few statements. The database rejects an event with no
// partition, which is reported as ErrInvalid.
func (s *Ingest) Store(ctx context.Context, inputs []EventInput) (*Result, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: no events", ErrInvalid)
	}

	events := make([]*models.Event, len(inputs))
	keys := make([]string, len(inputs))

	oldest, newest := inputs[0].OccurredAt, inputs[0].OccurredAt

	for i, input := range inputs {
		events[i] = toEvent(input)
		keys[i] = events[i].ID

		if input.OccurredAt.Before(oldest) {
			oldest = input.OccurredAt
		}

		if input.OccurredAt.After(newest) {
			newest = input.OccurredAt
		}
	}

	result := &Result{IDs: keys}
	err := s.db.Tx(ctx, func(tx *models.AppDB) error {
		// The time range reaches only the partitions these events can be in.
		seen, err := ant.Pluck(
			ctx,
			tx.Events.Where(
				ant.In(&models.Events.ID, keys...),
				ant.Between(
					&models.Events.OccurredAt,
					oldest.Add(-time.Second),
					newest.Add(time.Second),
				),
			),
			&models.Events.ID,
		)
		if err != nil {
			return err
		}

		known := make(map[string]bool, len(seen))

		for _, id := range seen {
			known[id] = true
		}

		fresh := make([]*models.Event, 0, len(events))

		for _, event := range events {
			if known[event.ID] {
				result.Duplicates++

				continue
			}

			known[event.ID] = true
			fresh = append(fresh, event)
		}

		result.Stored = len(fresh)

		if len(fresh) == 0 {
			return nil
		}

		if err := tx.Events.Insert(ctx, fresh...); err != nil {
			return err
		}

		return updateTotals(ctx, tx, fresh)
	})

	var invalid *ant.ValidationError

	switch {
	case errors.As(err, &invalid):
		return nil, fmt.Errorf("%w: %s %v", ErrInvalid, invalid.Field, invalid.Err)
	case ant.SQLState(err) == "23514":
		return nil, fmt.Errorf("%w: an event is outside the months this service keeps", ErrInvalid)
	case err != nil:
		return nil, err
	}

	return result, nil
}

// updateTotals keeps the actors and daily counts current, in the same transaction
// as the events, so they can never disagree. ON CONFLICT cannot touch one row twice
// in a statement, so rows are added up first.
func updateTotals(ctx context.Context, tx *models.AppDB, events []*models.Event) error {
	type dayKey struct {
		day                     time.Time
		service, action, result string
	}

	days := map[dayKey]*models.DailyCount{}
	actors := map[string]*models.Actor{}

	for _, e := range events {
		key := dayKey{e.OccurredAt.Truncate(24 * time.Hour), e.Service, e.Action, e.Outcome}
		if days[key] == nil {
			days[key] = &models.DailyCount{
				Day:     key.day,
				Service: e.Service,
				Action:  e.Action,
				Outcome: e.Outcome,
			}
		}

		days[key].Count++

		actor := actors[e.ActorID]
		if actor == nil {
			actor = &models.Actor{
				ID:        e.ActorID,
				Name:      e.ActorName,
				FirstSeen: e.OccurredAt,
				LastSeen:  e.OccurredAt,
			}
			actors[e.ActorID] = actor
		}

		actor.EventCount++

		if e.OccurredAt.Before(actor.FirstSeen) {
			actor.FirstSeen = e.OccurredAt
		}

		if e.OccurredAt.After(actor.LastSeen) {
			actor.LastSeen = e.OccurredAt
		}
	}

	counts := make([]*models.DailyCount, 0, len(days))

	for _, row := range days {
		counts = append(counts, row)
	}

	addCount := ant.OnConflict(&models.DailyCounts.Day).
		And(ant.F(&models.DailyCounts.Service)).
		And(ant.F(&models.DailyCounts.Action)).
		And(ant.F(&models.DailyCounts.Outcome)).
		Set(ant.SetExpr(
			&models.DailyCounts.Count,
			ant.SQL[int64]("daily_counts.count + excluded.count"),
		))

	if _, err := tx.DailyCounts.InsertMany(ctx, counts, ant.WithConflict(addCount)); err != nil {
		return err
	}

	people := make([]*models.Actor, 0, len(actors))

	for _, actor := range actors {
		people = append(people, actor)
	}

	addActor := ant.OnConflict(&models.Actors.ID).Set(
		ant.SetExpr(&models.Actors.Name, ant.SQL[string]("excluded.name")),
		ant.SetExpr(&models.Actors.FirstSeen, ant.SQL[time.Time]("LEAST(actors.first_seen, excluded.first_seen)")),
		ant.SetExpr(&models.Actors.LastSeen, ant.SQL[time.Time]("GREATEST(actors.last_seen, excluded.last_seen)")),
		ant.SetExpr(&models.Actors.EventCount, ant.SQL[int64]("actors.event_count + excluded.event_count")),
	)
	_, err := tx.Actors.InsertMany(ctx, people, ant.WithConflict(addActor))

	return err
}
