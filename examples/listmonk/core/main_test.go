package core_test

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/anttest"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

// shared is the database listmonk's own --install created, opened once.
var shared *models.AppDB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println(
			"TEST_DATABASE_URL is not set: the core tests need a database made by listmonk --install",
		)

		os.Exit(0)
	}

	db, err := ant.Open[models.AppDB](context.Background(), postgres.Dialect, dsn)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	shared = db
	code := m.Run()
	_ = db.Close()

	os.Exit(code)
}

// setup returns a core on a transaction that rolls back when the test ends.
func setup(t *testing.T) (*core.Core, *models.AppDB) {
	t.Helper()

	// A session per test: Isolate's rollback undoes the rows, but entities
	// tracked in a nested Tx stay in the tracker they were loaded into, and a
	// later SaveChanges on a shared tracker writes them again.
	db := anttest.Isolate(t, ant.Session(shared))

	return core.New(db), db
}

// upstream returns listmonk's own SQL for a named query, from the queries.sql
// that scripts/fetch-upstream.sh downloads. The test that compares with it
// skips when the file is missing.
func upstream(t *testing.T, name string) string {
	t.Helper()

	f, err := os.Open("../upstream/queries.sql")
	if err != nil {
		t.Skip("upstream/queries.sql is missing: run scripts/fetch-upstream.sh")
	}

	defer f.Close()

	var (
		body    strings.Builder
		reading bool
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if after, ok := strings.CutPrefix(line, "-- name: "); ok {
			if reading {
				break
			}

			reading = strings.TrimSpace(after) == name

			continue
		}

		if reading {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}

	if !reading {
		t.Fatalf("no query %q in upstream/queries.sql", name)
	}

	return body.String()
}

// seed adds lists and subscribers beyond what --install made: the install has
// two lists and two subscribers.
type seeded struct {
	Lists       []*models.List
	Subscribers []*models.Subscriber
}

func seed(t *testing.T, c *core.Core) seeded {
	t.Helper()

	ctx := t.Context()
	db := c.DB()

	var s seeded

	for i, l := range []struct {
		name  string
		typ   models.ListType
		optin models.ListOptin
	}{
		{"Weekly", models.ListPublic, models.OptinSingle},
		{"Product news", models.ListPublic, models.OptinDouble},
		{"Staff", models.ListPrivate, models.OptinSingle},
	} {
		list := &models.List{
			Name:  l.name,
			Type:  l.typ,
			Optin: l.optin,
			Tags:  []string{fmt.Sprintf("tag%d", i)},
		}

		if err := db.Lists.Insert(ctx, list); err != nil {
			t.Fatal(err)
		}

		s.Lists = append(s.Lists, list)
	}

	cities := []string{"Riga", "Tallinn", "Vilnius", "Riga", "Bengaluru"}
	for i, city := range cities {
		sub := core.NewSubscriber(
			fmt.Sprintf("user%d@example.org", i),
			fmt.Sprintf("User %c", 'A'+i),
			map[string]any{"city": city, "n": i},
		)

		lists := []int32{s.Lists[i%2].ID}

		if i == 4 {
			lists = append(lists, s.Lists[2].ID)
		}

		if err := c.InsertSubscriber(ctx, sub, lists, models.SubscriptionConfirmed); err != nil {
			t.Fatal(err)
		}

		s.Subscribers = append(s.Subscribers, sub)
	}

	return s
}

func sqlNamed(name string, v any) any { return sql.Named(name, v) }
