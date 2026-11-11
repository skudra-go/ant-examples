package services

import (
	"context"
	"fmt"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/audit/models"
)

// Stats reports on many events at once.
type Stats struct {
	db *models.AppDB
}

// NewStats returns the service.
func NewStats(db *models.AppDB) *Stats { return &Stats{db} }

// Bucket is one period of activity.
type Bucket struct {
	Start   time.Time `json:"start"`
	Total   int64     `json:"total"`
	Denied  int64     `json:"denied"`
	Failed  int64     `json:"failed"`
	Persons int64     `json:"persons"`
}

// Buckets is the probe for Bucket.
var Buckets = ant.Fields[Bucket]()

// units holds the groupings Activity allows. The unit is written into the SQL and
// not bound as a value, because PostgreSQL treats date_trunc($1, x) in SELECT and
// date_trunc($2, x) in GROUP BY as different expressions, and refuses the query. It
// comes from this list, never from a request.
var units = map[string]string{
	"hour":  "date_trunc('hour', ?)",
	"day":   "date_trunc('day', ?)",
	"week":  "date_trunc('week', ?)",
	"month": "date_trunc('month', ?)",
}

// Activity groups live events by hour, day, week, or month, with conditional counts
// in one pass: FILTER, not one query per outcome.
func (s *Stats) Activity(ctx context.Context, unit string, from, to time.Time) ([]Bucket, error) {
	template, ok := units[unit]
	if !ok {
		return nil, fmt.Errorf("%w: unit is hour, day, week, or month", ErrInvalid)
	}

	start := ant.SQL[time.Time](template, ant.Col(&models.Events.OccurredAt))

	return ant.Project[Bucket](
		s.db.Events.Where(
			ant.Gte(&models.Events.OccurredAt, from),
			ant.Lt(&models.Events.OccurredAt, to),
		).
			LeftJoinOn[models.Person](
			ant.EqCol(&models.Persons.EventID, &models.Events.ID),
			ant.EqCol(&models.Persons.OccurredAt, &models.Events.OccurredAt),
		).
			GroupByExpr(start).
			Order(start.Asc()),
		ant.IntoExpr(&Buckets.Start, start),
		ant.IntoExpr(&Buckets.Total, ant.CountDistinct(&models.Events.ID)),
		ant.IntoExpr(
			&Buckets.Denied,
			ant.CountDistinct(&models.Events.ID).
				Filter(ant.Eq(&models.Events.Outcome, models.OutcomeDenied)),
		),
		ant.IntoExpr(
			&Buckets.Failed,
			ant.CountDistinct(&models.Events.ID).
				Filter(ant.Eq(&models.Events.Outcome, models.OutcomeFailure)),
		),
		ant.IntoExpr(&Buckets.Persons, ant.CountDistinct(&models.Persons.Ref)),
	).All(ctx)
}

// Total is one line of the running totals.
type Total struct {
	Service string `json:"service"`
	Action  string `json:"action"`
	Count   int64  `json:"count"`
	Share   int64  `json:"sharePercent"`
}

// Totals is the probe for Total.
var Totals = ant.Fields[Total]()

// Rollup reads the running totals, which survive after their partitions are gone.
// A window over the aggregate gives each action's share of its service, in SQL.
func (s *Stats) Rollup(ctx context.Context, from, to time.Time) ([]Total, error) {
	counted := ant.Sum(&models.DailyCounts.Count)

	share := ant.SQL[int64](
		"(100 * SUM(?) / SUM(SUM(?)) OVER (PARTITION BY ?))::bigint",
		ant.Col(&models.DailyCounts.Count),
		ant.Col(&models.DailyCounts.Count),
		ant.Col(&models.DailyCounts.Service),
	)

	return ant.Project[Total](
		ant.Then(
			s.db.DailyCounts.Where(
				ant.Gte(&models.DailyCounts.Day, from),
				ant.Lt(&models.DailyCounts.Day, to),
			).
				GroupBy(&models.DailyCounts.Service),
			&models.DailyCounts.Action,
		).
			Order(ant.Col(&models.DailyCounts.Service).Asc(), counted.Desc()),
		ant.Into(&Totals.Service, &models.DailyCounts.Service),
		ant.Into(&Totals.Action, &models.DailyCounts.Action),
		ant.IntoExpr(&Totals.Count, counted),
		ant.IntoExpr(&Totals.Share, share),
	).All(ctx)
}

// ActorRow is one actor in a ranking.
type ActorRow struct {
	Actor   string `json:"actor"`
	Events  int64  `json:"events"`
	Persons int64  `json:"persons"`
	Rank    int64  `json:"rank"`
}

// ActorRows is the probe for ActorRow.
var ActorRows = ant.Fields[ActorRow]()

// TopActors ranks actors by how many people they touched. RANK runs over the
// grouped rows, so ties share a place.
func (s *Stats) TopActors(ctx context.Context, from, to time.Time, limit int) ([]ActorRow, error) {
	persons := ant.CountDistinct(&models.Persons.Ref)

	return ant.Project[ActorRow](
		s.db.Events.Where(
			ant.Gte(&models.Events.OccurredAt, from),
			ant.Lt(&models.Events.OccurredAt, to),
		).
			JoinOn[models.Person](
			ant.EqCol(&models.Persons.EventID, &models.Events.ID),
			ant.EqCol(&models.Persons.OccurredAt, &models.Events.OccurredAt),
		).
			GroupBy(&models.Events.ActorID).
			Order(persons.Desc()).
			Limit(limit),
		ant.Into(&ActorRows.Actor, &models.Events.ActorID),
		ant.IntoExpr(&ActorRows.Events, ant.CountDistinct(&models.Events.ID)),
		ant.IntoExpr(&ActorRows.Persons, persons),
		ant.IntoExpr(&ActorRows.Rank, ant.Rank().Over(ant.OrderedBy(persons.Desc()))),
	).All(ctx)
}

// Finding is an actor worth a look, and why.
type Finding struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

// Findings is the probe for Finding.
var Findings = ant.Fields[Finding]()

// Anomalies finds two kinds of behaviour, and combines them.
//
// Broad access: an actor who touched more distinct people than limit, found with
// HAVING over a join. Probing: an actor with denied events and no successful ones,
// found with EXISTS and NOT EXISTS on the actors table, which keeps its rows after
// the events are gone.
func (s *Stats) Anomalies(ctx context.Context, from, to time.Time, limit int64) ([]Finding, error) {
	inRange := []ant.Cond{
		ant.Gte(&models.Events.OccurredAt, from),
		ant.Lt(&models.Events.OccurredAt, to),
	}

	persons := ant.CountDistinct(&models.Persons.Ref)

	broad, err := ant.Project[Finding](
		s.db.Events.Where(inRange...).
			JoinOn[models.Person](
			ant.EqCol(&models.Persons.EventID, &models.Events.ID),
			ant.EqCol(&models.Persons.OccurredAt, &models.Events.OccurredAt),
		).
			GroupBy(&models.Events.ActorID).
			Having(persons.Gt(limit)).
			Order(persons.Desc()),
		ant.Into(&Findings.Actor, &models.Events.ActorID),
		ant.IntoExpr(&Findings.Reason, ant.SQL[string]("'broad access'")),
		ant.IntoExpr(&Findings.Count, persons),
	).All(ctx)
	if err != nil {
		return nil, err
	}

	ofActor := func(outcome string) *ant.Query[models.Event] {
		return s.db.Events.Where(append(inRange,
			ant.EqCol(&models.Events.ActorID, &models.Actors.ID),
			ant.Eq(&models.Events.Outcome, outcome))...)
	}

	probing, err := s.db.Actors.Where(
		ant.Exists(ofActor(models.OutcomeDenied)),
		ant.NotExists(ofActor(models.OutcomeSuccess)),
	).
		OrderBy(&models.Actors.ID).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for _, actor := range probing {
		broad = append(
			broad,
			Finding{Actor: actor.ID, Reason: "only denied", Count: actor.EventCount},
		)
	}

	return broad, nil
}

// LatestEvent is an actor with their newest event.
type LatestEvent struct {
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	At     time.Time `json:"at"`
}

// LatestByActor reads each actor's newest event. A lateral join runs its subquery
// once per actor, each using the actor index and stopping at one row, which is the
// fast way to read the top of many groups.
func (s *Stats) LatestByActor(ctx context.Context, limit int) ([]LatestEvent, error) {
	newest := s.db.Events.Where(ant.EqCol(&models.Events.ActorID, &models.Actors.ID)).
		OrderByDesc(&models.Events.OccurredAt).Limit(1)

	return ant.JoinLateral(s.db.Actors.OrderByDesc(&models.Actors.LastSeen).Limit(limit), newest).
		Select(func(a *models.Actor, e *models.Event) LatestEvent {
			return LatestEvent{Actor: a.ID, Action: e.Action, At: e.OccurredAt}
		}).All(ctx)
}
