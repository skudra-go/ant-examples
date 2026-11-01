package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/audit/models"
)

// lockKey is the advisory lock that lets one instance run retention at a time.
const lockKey = 7_401_001

// Retention keeps the partitions in order: future months ready, old months gone.
// Dropping a partition takes a moment and frees its space at once, where deleting
// a month's rows would take hours and leave the table bloated.
type Retention struct {
	db      *models.AppDB
	log     *slog.Logger
	keep    int
	ahead   int
	events  *postgres.Partitions
	persons *postgres.Partitions
}

// NewRetention returns the service.
func NewRetention(db *models.AppDB, log *slog.Logger, keepMonths, ahead int) (*Retention, error) {
	events, err := postgres.MonthlyPartitions[models.Event](db)
	if err != nil {
		return nil, err
	}

	persons, err := postgres.MonthlyPartitions[models.Person](db)
	if err != nil {
		return nil, err
	}

	return &Retention{db, log, keepMonths, ahead, events, persons}, nil
}

// Report says what a run did.
type Report struct {
	Skipped bool      `json:"skipped,omitempty"`
	Created []string  `json:"created"`
	Dropped []string  `json:"dropped"`
	Cutoff  time.Time `json:"cutoff"`
}

// Prepare creates this month's partitions and the next ones. An event outside
// them has nowhere to go, so this runs at startup too.
func (r *Retention) Prepare(ctx context.Context, from time.Time, months int) ([]string, error) {
	created, err := r.events.Ensure(ctx, from, months)
	if err != nil {
		return created, err
	}

	more, err := r.persons.Ensure(ctx, from, months)

	return append(created, more...), err
}

// Run creates upcoming months and drops expired ones. Each instance schedules it,
// and an advisory lock on one connection lets one of them work, which is why this
// holds a connection through ant.UseConn: the lock belongs to the session.
func (r *Retention) Run(ctx context.Context, now time.Time) (*Report, error) {
	conn, err := r.db.DB().Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	session, err := ant.UseConn(r.db, conn)
	if err != nil {
		return nil, err
	}

	locked, err := ant.Raw[bool](session, "SELECT pg_try_advisory_lock(?)", lockKey).First(ctx)
	if err != nil {
		return nil, err
	}

	if !*locked {
		return &Report{Skipped: true}, nil
	}

	defer func(conn *sql.Conn) {
		_, _ = conn.ExecContext(
			context.WithoutCancel(ctx),
			"SELECT pg_advisory_unlock($1)",
			lockKey,
		)
	}(conn)

	report := &Report{Cutoff: r.Cutoff(now), Created: []string{}, Dropped: []string{}}
	created, err := r.Prepare(ctx, now, r.ahead)
	report.Created = append(report.Created, created...)

	if err != nil {
		return report, err
	}
	// Persons first: they refer to events, and a partition others refer to cannot go.
	dropped, err := r.persons.DropBefore(ctx, report.Cutoff)
	report.Dropped = append(report.Dropped, dropped...)

	if err != nil {
		return report, fmt.Errorf("drop persons: %w", err)
	}

	dropped, err = r.events.DropBefore(ctx, report.Cutoff)
	report.Dropped = append(report.Dropped, dropped...)

	if err != nil {
		return report, fmt.Errorf("drop events: %w", err)
	}

	r.log.Info(
		"retention finished",
		"created",
		len(report.Created),
		"dropped",
		len(report.Dropped),
		"cutoff",
		report.Cutoff,
	)

	return report, nil
}

// Cutoff is the start of the oldest month kept. Whole months older go.
func (r *Retention) Cutoff(now time.Time) time.Time {
	now = now.UTC()

	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -r.keep, 0)
}

// MonthRow is one partition and what is in it.
type MonthRow struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
	Size  string `json:"size"`
}

// Months lists the partitions, with their sizes, by asking the database.
func (r *Retention) Months(ctx context.Context) ([]MonthRow, error) {
	return ant.Raw[MonthRow](r.db, `
		SELECT c.relname AS table,
		       GREATEST(c.reltuples, 0)::bigint AS rows,
		       pg_size_pretty(pg_total_relation_size(c.oid)) AS size
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent IN ('events'::regclass, 'event_persons'::regclass)
		ORDER BY c.relname`).All(ctx)
}
