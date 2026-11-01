package migrations

import (
	"context"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/blog/models"
)

func init() {
	migrate.Seed[models.AppDB]("demo", demo)
}

// demo fills a new database with something to look at. It runs on every
// docker compose up, so each insert checks first, and running it twice changes
// nothing.
func demo(ctx context.Context, db *models.AppDB) error {
	grace, _, err := db.Users.FirstOrCreate(
		ctx,
		&models.User{
			Name:  "Grace",
			Email: "grace@example.com",
			About: new("Wrote the first compiler."),
		},
		ant.Eq(&models.Users.Email, "grace@example.com"),
	)
	if err != nil {
		return err
	}

	tag, _, err := db.Tags.FirstOrCreate(
		ctx,
		&models.Tag{Name: "go"},
		ant.Eq(&models.Tags.Name, "go"),
	)
	if err != nil {
		return err
	}

	post, created, err := db.Posts.FirstOrCreate(
		ctx,
		&models.Post{
			AuthorID:    grace.ID,
			Title:       "Hello, ant",
			Slug:        "hello-ant",
			Body:        "This post came from a seeder.",
			PublishedAt: new(time.Now().UTC()),
		},
		ant.Eq(&models.Posts.Slug, "hello-ant"),
	)
	if err != nil {
		return err
	}

	if !created {
		return nil
	}

	return ant.Link(ctx, db, post, &models.Posts.Tags, tag)
}
