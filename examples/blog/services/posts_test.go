package services_test

import (
	"context"
	"errors"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/blog/migrations"
	"github.com/skudra-go/ant-examples/blog/models"
	"github.com/skudra-go/ant-examples/blog/services"
)

// open returns a database built by the real migrations, so a broken migration
// fails here first. Each test gets a schema of its own on TEST_DATABASE_URL, and
// the test skips without one, so go test ./... works with nothing running.
func open(t *testing.T) *models.AppDB {
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

	const schema = "test_services"
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

	return db
}

func TestPostLifecycle(t *testing.T) {
	db := open(t)
	ctx := t.Context()
	posts := services.NewPosts(db)

	user, err := services.NewUsers(db).Create(ctx, "Ada", "ada@example.com")
	if err != nil {
		t.Fatal(err)
	}

	post, err := posts.Create(ctx, services.PostInput{
		AuthorID: user.ID,
		Title:    "Hello, World!",
		Body:     "Hi",
		Tags:     []string{"Go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if post.Slug != "hello-world" || len(post.Tags) != 1 || post.Author == nil {
		t.Fatalf("post = %+v", post)
	}

	// A draft stays hidden until published.
	if _, err := posts.Get(ctx, post.Slug); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("draft: %v", err)
	}

	if err := posts.Publish(ctx, post.Slug); err != nil {
		t.Fatal(err)
	}

	if _, err := services.NewComments(db).Add(ctx, post.Slug, "Grace", "Nice."); err != nil {
		t.Fatal(err)
	}

	got, err := posts.Get(ctx, post.Slug)
	if err != nil {
		t.Fatal(err)
	}

	if got.Author.Name != "Ada" || len(got.Tags) != 1 || got.CommentCount != 1 ||
		len(got.Comments) != 1 {
		t.Fatalf("post = %+v", got)
	}

	// An edit is stored, not just returned, and only what changed moves.
	title := "A New Title"

	updated, err := posts.Update(
		ctx,
		post.Slug,
		services.PostChanges{Title: &title, Tags: []string{"go", "ant"}},
	)
	if err != nil {
		t.Fatal(err)
	}

	if updated.Title != title || len(updated.Tags) != 2 || updated.Author == nil {
		t.Fatalf("updated = %+v", updated)
	}

	stored, err := posts.Get(ctx, post.Slug)
	if err != nil {
		t.Fatal(err)
	}

	if stored.Title != title || stored.Body != "Hi" {
		t.Fatalf("stored = %+v", stored)
	}

	// The same title again is a conflict, with nothing left behind.
	if _, err := posts.Create(
		ctx,
		services.PostInput{AuthorID: user.ID, Title: "Hello, World!"},
	); !errors.Is(
		err,
		services.ErrConflict,
	) {
		t.Fatalf("duplicate: %v", err)
	}

	// Deleting hides it from every query.
	if err := posts.Delete(ctx, post.Slug); err != nil {
		t.Fatal(err)
	}

	if _, err := posts.Get(ctx, post.Slug); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}
