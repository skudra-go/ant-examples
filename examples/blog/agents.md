# Agent guide

A JSON API for a blog, built on the ant ORM, in Go 1.27. The README explains the layout and why.

## Commands

```sh
docker compose up --build                 # run everything
docker compose up -d db                   # just the database
export DATABASE_URL='postgres://blog:blog@localhost:55440/blog?sslmode=disable'

go build ./... && go vet ./...
TEST_DATABASE_URL="$DATABASE_URL" go test ./...

go run ./cmd/migrate diff <name>          # after changing a struct
go run ./cmd/migrate up
go run ./cmd/migrate check                # models, snapshot, and files agree
go run ./cmd/migrate drift                # the database matches
```

## Rules

Layers: `routes` calls `services`, which calls `models`. Routes hold no logic, and services know nothing about HTTP. `models` imports nothing from this project.

Environment variables are read in `config.go`, and nowhere else.

Changing the schema:

- Change the struct, then run `diff`. Never write a schema change by hand when `diff` can.
- Read the generated file, and act on each `WARNING`. A column with existing data often needs its backfill edited.
- A rename needs `--rename table.old=new`, or the column is dropped with its data.
- Never edit a migration that has been applied or merged. Write a new one.
- `migrations/versions/schema.snapshot.json` is generated. Never edit it. If it conflicts in a merge, take either side, then run `snapshot`.
- `check` and `drift` must both pass before you finish.
- Views and functions go in `migrations/code`, and must be safe to run again.
- Go data migrations use `ant.Raw`, not models, because models change later.

Using ant:

- Column references are probes: `&models.Posts.Slug`. There are no column name strings in queries.
- `app.DB` has tracking off, because it is shared. For `SaveChanges`, use `ant.Session(db, ant.WithTracking(true))`, once per unit of work.
- Load relations with `Include` and `IncludeOne`. A relation that was not loaded is nil, and there is no lazy loading.
- `Delete` on `Post` is a soft delete, and queries skip those rows.
