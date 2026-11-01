# Agent guide

A double-entry ledger API on the ant ORM, in Go 1.27. The README explains the design.

## Commands

```sh
docker compose up --build                 # run everything
docker compose up -d db                   # just the database
export DATABASE_URL='postgres://ledger:ledger@localhost:55442/ledger?sslmode=disable'

go build ./... && go vet ./...
TEST_DATABASE_URL="$DATABASE_URL" go test ./...

go run ./cmd/migrate diff <name>          # after changing a struct
go run ./cmd/migrate up
go run ./cmd/migrate check
go run ./cmd/migrate drift
```

## Rules

Money:

- Amounts are `models.Money`, an integer of cents. Never use a float.
- Balances change only in `Journal.post`. Nothing else writes `balance`.
- Entries and lines are never edited or deleted. A correction is a new entry, through `Journal.Reverse`.
- Lock accounts in key order, as `post` does, or two requests can deadlock.
- Rules that protect money go in the database too: a check constraint, or a trigger in `migrations/code`.

Transactions:

- A function passed to `Tx` can run more than once. It must not touch anything outside the database.
- Tell the outside world through the outbox, in the same transaction.
- Use `ant.Session(db, ant.WithTracking(true))` for `SaveChanges`, once per unit of work. The shared context does not track.

Tenants:

- A model owned by a tenant embeds `models.Tenanted`.
- A context needs a tenant for those models. Do not work around the error by ignoring the filter.
- `ant.Raw` does not apply the filter, so a raw query must name the tenant itself.

Modeling:

- Do not put `.Default` on a field whose zero value means something: a zero field is left to the database.

Changing the schema: change the struct, run `diff`, read the file, and act on each `WARNING`. Never edit a migration that has run. `check` and `drift` must pass before you finish.
