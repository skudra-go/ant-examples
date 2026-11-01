# Ledger

A double-entry ledger, as a multi-tenant API. Money is the domain where "mostly right" is wrong: two requests must never overdraw an account between them, a crash must never lose a message, and one customer must never see another's books. This example shows how ant holds up where correctness is the whole job.

## Run it

```sh
docker compose up --build
```

That starts PostgreSQL, applies the migrations, adds a tenant with a small chart of accounts, and serves the API on port 8082, with two workers sharing the outbox.

```sh
H='Authorization: Bearer dev-token'; X='X-Tenant: 1'

# Put money in, then move some. A key makes the request safe to repeat.
curl -X POST localhost:8082/transfers -H "$H" -H "$X" \
  -d '{"from":"equity","to":"assets:bank","amount":"1000.00","memo":"Capital","key":"fund-1"}'
curl -X POST localhost:8082/transfers -H "$H" -H "$X" \
  -d '{"from":"assets:bank","to":"assets:savings","amount":"250.50","memo":"Savings"}'

# See what a transfer would do, for real, and leave nothing behind.
curl -X POST localhost:8082/transfers/preview -H "$H" -H "$X" \
  -d '{"from":"assets:savings","to":"expenses","amount":"10.00","memo":"Maybe"}'

# Too much: refused by a database constraint, with a 422.
curl -X POST localhost:8082/transfers -H "$H" -H "$X" \
  -d '{"from":"assets:savings","to":"expenses","amount":"9999.00","memo":"No"}'

curl localhost:8082/accounts -H "$H" -H "$X"                          # the chart, with rolled-up totals
curl localhost:8082/accounts/assets:bank/statement -H "$H" -H "$X"    # lines, with a running balance
curl localhost:8082/reports/trial-balance -H "$H" -H "$X"             # do the books balance?
```

`docker compose logs worker` shows both workers sending what the transfers wrote to the outbox.

## What it shows

| Problem | How it is solved | Where |
| --- | --- | --- |
| Two transfers race over one balance | Accounts lock in key order with `ForUpdate`, so requests queue and cannot deadlock. `ant.Attempts` retries the rare collision | [journal.go](services/journal.go) |
| A bug tries to overdraw | A check constraint, `balance >= floor`, refuses it whatever the Go code does | [account.go](models/account.go) |
| An entry that does not add up | `Validate` catches it in Go, and a deferred trigger catches it at commit, even from `psql` | [entry.go](models/entry.go), [10_ledger_rules.sql](migrations/code/10_ledger_rules.sql) |
| Someone edits history | A trigger makes `lines` append-only. A correction is a new entry | [10_ledger_rules.sql](migrations/code/10_ledger_rules.sql) |
| A client retries after a timeout | An idempotency key returns the first result | [journal.go](services/journal.go) |
| A message lost in a crash, or sent for a rollback | The outbox row commits with the money, and a worker sends it | [outbox.go](services/outbox.go) |
| Many workers, one queue | `ForUpdate().SkipLocked()` gives each its own batch | [outbox.go](services/outbox.go) |
| One tenant seeing another | An embedded struct adds a global filter that reaches joins and subqueries, and fails if the context has no tenant | [tenant.go](models/tenant.go) |
| Stale edits | `ant:"version"` makes the `UPDATE` match no row, and the API answers 409 | [accounts.go](services/accounts.go) |
| Bank data at rest | `secure.Value` encrypts the IBAN, with key rotation, and it cannot reach a log or a JSON response | [account.go](models/account.go) |
| "What would this do?" | `ant.DryRun` runs the real code in a transaction and rolls it back | [journal.go](services/journal.go) |
| A chart of accounts sent as one tree | `TrackGraph` decides per node, and `SaveChanges` orders the inserts and fills parent keys | [accounts.go](services/accounts.go) |
| A bank file that arrives again, corrected | `ant.Merge` over `ant.Values` updates or inserts each line in one statement | [reconcile.go](services/reconcile.go) |
| Reports | `Recursive` rolls up a subtree, a window gives a running balance, `Filter` splits debits from credits | [reports.go](services/reports.go) |

## How a transfer works

Everything goes through `Journal.Post`, the one place balances change. It reads like plain Go, and that is the point of a unit of work:

```go
accounts, err := tx.Accounts.Where(ant.In(&models.Accounts.Code, codes...)).
    OrderBy(&models.Accounts.ID).ForUpdate().All(ctx)   // lock, in a fixed order

for _, line := range input.Lines {
    account := byCode[line.Account]
    account.Balance += line.Amount                      // change the struct
    entry.Lines = append(entry.Lines, models.Line{AccountID: account.ID, Amount: line.Amount})
}

ant.Add(tx, entry)
ant.Add(tx, message)                                    // the outbox row
tx.SaveChanges(ctx)                                     // one transaction, in the right order
```

`SaveChanges` writes the entry first, because its lines need its key, then the lines, then an update per changed balance, and the version on each account goes up. If anything fails, nothing happened.

A database rule then decides what Go cannot be trusted to. The concurrency test runs fifty transfers against accounts that can afford only some of them, and checks that none went negative and the total is unchanged to the cent.

## Multi-tenancy

Every tenant-owned model embeds `models.Tenanted`:

```go
type Account struct {
    models.Tenanted
    ...
}
```

That one line brings a column, a global filter, and the tenant on insert, so a query cannot forget it. The filter reads the tenant from the context, and reaches joins, `Include`, `Has`, and subqueries by itself. With no tenant in the context, the query fails. It does not quietly return everything, which is the mistake that makes a filter like this dangerous.

## Money

`models.Money` is an `int64` of cents, in Go and in the database, because a float cannot hold 0.10. JSON shows it as a string, `"12.34"`, since most JSON readers would turn a number into a float.

## Good to know

**`.Default` means zero is left to the database.** This example learned that the hard way: a column default on `floor` silently replaced a real floor of `0.00`, so accounts could be overdrawn. A field whose zero value means something must not have a default. `Floor` has none, and `NoFloor` says "no limit" out loud.

**A rolled-back preview still uses a sequence number**, because PostgreSQL does not give them back. Entry numbers can have gaps. Accountants who need none should number at period close.

**`ant.WithTracking(false)` on the shared context, `ant.Session` per unit of work.** A tracker keeps what it loads, so the shared context does not track, and each write opens a session with `ant.WithTracking(true)`.

**`Tx` runs its function again on a retry**, so it must not do anything outside the database. That is one more reason the outbox exists.

## Layout

The same as the [blog](../blog/README.md#layout), with `cmd/worker` beside `cmd/server`. Rules the database enforces live in `migrations/code`, where they rerun when edited.

## Configuration

| Variable | Required | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `API_TOKEN` | yes | Guards every route |
| `ENCRYPTION_KEY` | yes | 32 bytes in base64, for IBANs |
| `PORT` | no | Defaults to 8080 |
| `LOG_LEVEL` | no | `debug` logs every statement, and what each save changes |

## Tests

```sh
docker compose up -d db
TEST_DATABASE_URL='postgres://ledger:ledger@localhost:55442/ledger?sslmode=disable' go test ./...
```

They run on a schema built by the real migrations, and cover racing transfers, rules enforced past Go, tenant isolation, workers sharing the outbox, and repeatable imports. [`anttest.NewFactory`](services/services_test.go) makes the accounts.
