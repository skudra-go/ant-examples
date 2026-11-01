// Command loadgen fills the database with months of plausible events, using COPY,
// which is how a backfill or a migration from another system would load them. It
// bypasses the API and its running totals on purpose, and adds them up afterwards
// with one statement.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/dialect/postgres/pgxext"
	"github.com/skudra-go/ant/ids"

	"github.com/skudra-go/ant-examples/audit/models"
	"github.com/skudra-go/ant-examples/audit/services"
)

func main() {
	count := flag.Int("events", 20000, "events to generate")
	months := flag.Int("months", 9, "months back to spread them over")

	flag.Parse()

	if err := run(*count, *months); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

var (
	services_ = []string{"billing", "crm", "support", "hr"}
	actions   = []string{
		"customer.viewed",
		"customer.updated",
		"invoice.exported",
		"login",
		"report.downloaded",
	}
	actors = []string{"ada", "grace", "linus", "edsger", "barbara", "ken"}
)

func run(count, months int) error {
	ctx := context.Background()

	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		os.Getenv("DATABASE_URL"),
		ant.WithTracking(false),
	)
	if err != nil {
		return err
	}
	defer db.Close()

	now := time.Now().UTC()

	retention, err := services.NewRetention(db, slog.Default(), months, 0)
	if err != nil {
		return err
	}

	if _, err := retention.Prepare(ctx, now.AddDate(0, -months, 0), months); err != nil {
		return err
	}

	events := make([]*models.Event, 0, count)

	var persons []*models.Person

	for i := range count {
		actor := actors[rand.IntN(len(actors))]

		span := int64(time.Duration(months) * 30 * 24 * time.Hour)
		at := now.Add(-time.Duration(rand.Int64N(span))).
			Truncate(time.Microsecond)

		outcome := models.OutcomeSuccess

		if rand.IntN(20) == 0 {
			outcome = models.OutcomeDenied
		}

		event := &models.Event{
			ID:           ids.UUID(),
			OccurredAt:   at,
			Service:      services_[rand.IntN(len(services_))],
			Action:       actions[rand.IntN(len(actions))],
			Outcome:      outcome,
			ActorID:      actor,
			ActorName:    actor,
			ResourceType: "customer",
			ResourceID:   fmt.Sprint(rand.IntN(5000)),
			Summary:      fmt.Sprintf("%s worked on customer %d", actor, i%5000),
			Metadata: ant.JSON[map[string]any]{
				V: map[string]any{"region": []string{"eu", "us"}[i%2]},
			},
		}
		events = append(events, event)

		for range 1 + rand.IntN(3) {
			persons = append(persons, &models.Person{
				EventID:    event.ID,
				OccurredAt: at,
				Ref:        fmt.Sprintf("person-%d", rand.IntN(2000)),
				Role:       models.RoleViewed,
			})
		}
	}

	started := time.Now()

	stored, err := pgxext.Copy(ctx, db, events)
	if err != nil {
		return err
	}

	linked, err := pgxext.Copy(ctx, db, persons)
	if err != nil {
		return err
	}

	if _, err := ant.Raw[any](db, totals).Exec(ctx); err != nil {
		return err
	}

	fmt.Printf(
		"Loaded %d events and %d persons in %s\n",
		stored,
		linked,
		time.Since(started).Round(time.Millisecond),
	)

	return nil
}

// totals rebuilds the running totals from the events.
const totals = `
	WITH wiped AS (DELETE FROM daily_counts)
	INSERT INTO daily_counts (day, service, action, outcome, count)
	SELECT occurred_at::date, service, action, outcome, count(*) FROM events GROUP BY 1, 2, 3, 4;
	INSERT INTO actors (id, name, first_seen, last_seen, event_count)
	SELECT actor_id, max(actor_name), min(occurred_at), max(occurred_at), count(*) FROM events GROUP BY 1
	ON CONFLICT (id) DO UPDATE SET first_seen = excluded.first_seen, last_seen = excluded.last_seen, event_count = excluded.event_count`
