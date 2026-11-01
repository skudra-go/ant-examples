# Agent guide

An audit log API on the ant ORM, in Go 1.27, with tables partitioned by month. The README explains the design.

## Commands

```sh
docker compose up --build                 # run everything
docker compose up -d db                   # just the database
export DATABASE_URL='postgres://audit:audit@localhost:55441/audit?sslmode=disable'

go build ./... && go vet ./...
TEST_DATABASE_URL="$DATABASE_URL" go test ./...

go run ./cmd/migrate diff <name>          # after changing a struct
go run ./cmd/migrate up
go run ./cmd/migrate check
go run ./cmd/migrate drift
go run ./cmd/loadgen --events 20000       # generated data
```

## Rules

Layers: `routes` calls `services`, which calls `models`. Environment variables are read in `config.go`, and nowhere else.

Partitioned tables:

- A partitioned table's primary key and unique constraints must include `occurred_at`.
- A row in `event_persons` carries its event's `occurred_at`. Set it whenever you make one.
- Joins between the two tables name both `event_id` and `occurred_at`, so PostgreSQL can pair partitions.
- Partitions are not migrations. Never write `CREATE TABLE ... PARTITION OF` in a migration: `services/retention.go` manages them.
- Drop `event_persons` partitions before `events` partitions.

Queries:

- Filter on `occurred_at` wherever you can, because it skips whole partitions.
- Never bind a value inside an expression that also appears in `GROUP BY`. PostgreSQL treats the two as different, so write it into the SQL from a fixed list.
- `app.DB` has tracking off. This service writes through `Insert`, `InsertMany`, and bulk updates.
- Text from a request never goes into SQL. `ant.SQL` takes it as an argument.

Changing the schema: change the struct, run `diff`, read the file, and act on each `WARNING`. Never edit a migration that has run. `check` and `drift` must pass before you finish.
