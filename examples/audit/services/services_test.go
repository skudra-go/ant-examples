package services_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/audit/migrations"
	"github.com/skudra-go/ant-examples/audit/models"
	"github.com/skudra-go/ant-examples/audit/services"
)

// open returns a database built by the real migrations, in a schema of its own on
// TEST_DATABASE_URL, with partitions around now. The test skips without one.
func open(t *testing.T) (*models.AppDB, *services.Retention) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()

	admin, err := ant.Open[models.AppDB](ctx, postgres.Dialect, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	const schema = "test_audit"
	if _, err := admin.DB().
		ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE; CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		dsn+"&search_path="+schema,
		ant.WithTracking(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = db.DB().ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		db.Close()
	})

	if err := migrate.Up(ctx, db, migrate.Config{FS: migrations.Files}); err != nil {
		t.Fatal(err)
	}

	retention, err := services.NewRetention(
		db,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		2,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := retention.Prepare(ctx, time.Now().AddDate(0, -6, 0), 8); err != nil {
		t.Fatal(err)
	}

	return db, retention
}

func input(actor, ref string, at time.Time) services.EventInput {
	return services.EventInput{
		OccurredAt: at,
		Service:    "crm",
		Action:     "customer.viewed",
		ActorID:    actor,
		ActorName:  actor,
		Summary:    "viewed a customer",
		Metadata:   map[string]any{"region": "eu"},
		Persons: []services.PersonInput{
			{Ref: ref, Role: models.RoleViewed, Fields: []string{"email"}},
		},
	}
}

func TestPersonsAreStoredApart(t *testing.T) {
	db, _ := open(t)
	ctx := t.Context()
	event := input("ada", "p-1", time.Now())
	event.Persons = append(
		event.Persons,
		services.PersonInput{Ref: "p-2", Role: models.RoleAffected},
	)

	result, err := services.NewIngest(db).Store(ctx, []services.EventInput{event})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := db.Persons.Where(ant.Eq(&models.Persons.EventID, result.IDs[0])).
		OrderBy(&models.Persons.Ref).
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 || rows[0].Ref != "p-1" ||
		!rows[0].OccurredAt.Equal(event.OccurredAt.UTC().Truncate(time.Microsecond)) {
		t.Fatalf("persons: %+v", rows)
	}
}

func TestRetriesAreSafe(t *testing.T) {
	db, _ := open(t)
	ctx := t.Context()
	ingest := services.NewIngest(db)
	event := input("ada", "p-1", time.Now())
	event.ID = "0192f3a4-7c3d-7b1e-8f00-000000000099"

	for range 3 {
		if _, err := ingest.Store(ctx, []services.EventInput{event}); err != nil {
			t.Fatal(err)
		}
	}

	count, err := db.Events.Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("events: %d, %v", count, err)
	}

	actor, err := db.Actors.Find(ctx, "ada")
	if err != nil || actor.EventCount != 1 {
		t.Fatalf("actor counted a duplicate: %+v, %v", actor, err)
	}
}

func TestInvalidEventsLeaveNothing(t *testing.T) {
	db, _ := open(t)
	ctx := t.Context()
	ingest := services.NewIngest(db)
	bad := input("ada", "p-1", time.Now())
	bad.Persons[0].Role = "nonsense"

	good := input("ada", "p-2", time.Now())
	if _, err := ingest.Store(
		ctx,
		[]services.EventInput{good, bad},
	); !errors.Is(
		err,
		services.ErrInvalid,
	) {
		t.Fatalf("bad role: %v", err)
	}

	if count, _ := db.Events.Count(ctx); count != 0 {
		t.Fatalf("%d events survived a failed batch", count)
	}

	if _, err := ingest.Store(
		ctx,
		[]services.EventInput{input("ada", "p-1", time.Now().AddDate(-5, 0, 0))},
	); !errors.Is(
		err,
		services.ErrInvalid,
	) {
		t.Fatalf("no partition: %v", err)
	}
}

func TestCursorWalksEveryEvent(t *testing.T) {
	db, _ := open(t)
	ctx := t.Context()

	var batch []services.EventInput

	for i := range 25 {
		batch = append(batch, input("ada", "p-1", time.Now().Add(-time.Duration(i)*time.Hour)))
	}

	if _, err := services.NewIngest(db).Store(ctx, batch); err != nil {
		t.Fatal(err)
	}

	search := services.NewSearch(db)
	seen, cursor := 0, ""

	for {
		page, err := search.Events(ctx, services.Filter{Person: "p-1"}, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}

		seen += len(page.Items)

		if page.Next == "" {
			break
		}

		if len(page.Items[0].Persons) != 1 || page.Items[0].PersonCount != 1 {
			t.Fatalf("persons: %+v", page.Items[0])
		}

		cursor = page.Next
	}

	if seen != 25 {
		t.Fatalf("saw %d of 25", seen)
	}
}

func TestStatistics(t *testing.T) {
	db, _ := open(t)
	ctx := t.Context()
	ingest := services.NewIngest(db)
	now := time.Now()
	denied := input("eve", "p-9", now)

	denied.Outcome = models.OutcomeDenied

	if _, err := ingest.Store(
		ctx,
		[]services.EventInput{input("ada", "p-1", now), input("ada", "p-2", now), denied},
	); err != nil {
		t.Fatal(err)
	}

	stats := services.NewStats(db)
	from, to := now.Add(-time.Hour), now.Add(time.Hour)

	buckets, err := stats.Activity(ctx, "day", from, to)
	if err != nil {
		t.Fatal(err)
	}

	var total, deniedCount int64

	for _, b := range buckets {
		total += b.Total
		deniedCount += b.Denied
	}

	if total != 3 || deniedCount != 1 {
		t.Fatalf("buckets: %+v", buckets)
	}

	if _, err := stats.Activity(
		ctx,
		"'; DROP TABLE events; --",
		from,
		to,
	); !errors.Is(
		err,
		services.ErrInvalid,
	) {
		t.Fatalf("a unit from outside the list: %v", err)
	}

	findings, err := stats.Anomalies(ctx, from, to, 1)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"ada": "broad access", "eve": "only denied"}

	if len(findings) != 2 {
		t.Fatalf("findings: %+v", findings)
	}

	for _, f := range findings {
		if want[f.Actor] != f.Reason {
			t.Fatalf("finding: %+v", f)
		}
	}

	top, err := stats.TopActors(ctx, from, to, 5)
	if err != nil || top[0].Actor != "ada" || top[0].Rank != 1 {
		t.Fatalf("top: %+v, %v", top, err)
	}

	rollup, err := stats.Rollup(ctx, from.AddDate(0, 0, -1), to.AddDate(0, 0, 1))
	if err != nil || rollup[0].Count != 3 {
		t.Fatalf("rollup: %+v, %v", rollup, err)
	}

	latest, err := stats.LatestByActor(ctx, 5)
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest: %+v, %v", latest, err)
	}
}

func TestErasureKeepsTheEvent(t *testing.T) {
	db, _ := open(t)
	ctx := t.Context()
	name := "Ann"
	event := input("ada", "p-1", time.Now())

	event.Persons[0].Name = &name

	if _, err := services.NewIngest(db).Store(ctx, []services.EventInput{event}); err != nil {
		t.Fatal(err)
	}

	persons := services.NewPersons(db)

	erasure, err := persons.Erase(ctx, "p-1", "dpo")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := persons.Summary(ctx, "p-1"); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("still findable: %v", err)
	}

	stored, err := db.Persons.Where(ant.Eq(&models.Persons.Ref, erasure.Pseudo)).First(ctx)
	if err != nil || stored.Name != nil || len(stored.Fields) != 0 {
		t.Fatalf("stored: %+v, %v", stored, err)
	}

	count, err := db.Events.Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("the event and the erasure record: %d, %v", count, err)
	}

	if _, err := persons.Erase(ctx, "p-1", "dpo"); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("erasing twice: %v", err)
	}
}

func TestRetention(t *testing.T) {
	db, retention := open(t)
	ctx := t.Context()
	now := time.Now().UTC()

	old := now.AddDate(0, -5, 0)
	events := []services.EventInput{input("ada", "p-1", now), input("ada", "p-2", old)}

	if _, err := services.NewIngest(db).Store(ctx, events); err != nil {
		t.Fatal(err)
	}

	report, err := retention.Run(ctx, now)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Dropped) == 0 {
		t.Fatalf("nothing dropped: %+v", report)
	}

	count, err := db.Events.Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("events left: %d, %v", count, err)
	}

	if persons, _ := db.Persons.Count(ctx); persons != 1 {
		t.Fatalf("persons left: %d", persons)
	}

	if actor, err := db.Actors.Find(ctx, "ada"); err != nil || actor.EventCount != 2 {
		t.Fatalf("the running total outlives the partition: %+v, %v", actor, err)
	}

	t.Run("a second run has nothing to do", func(t *testing.T) {
		report, err := retention.Run(ctx, now)
		if err != nil || len(report.Dropped) != 0 {
			t.Fatalf("%+v, %v", report, err)
		}
	})

	t.Run("two instances do not collide", func(t *testing.T) {
		errs := make(chan error, 4)

		for range 4 {
			go func() {
				_, err := retention.Run(ctx, now)
				errs <- err
			}()
		}

		for range 4 {
			if err := <-errs; err != nil {
				t.Fatal(fmt.Errorf("concurrent run: %w", err))
			}
		}
	})
}
