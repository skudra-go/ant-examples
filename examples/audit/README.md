# Audit

An audit log service. Other systems send it what happened, and who it happened to. It stores that for as long as the law asks, answers questions about it, and throws old months away cheaply.

This is the demanding example. Where the [blog](../blog/) shows a typical application, this one pushes on what is hard: tables split by month, keys that span columns, a high write rate, and queries that need real SQL.

## Run it

```sh
docker compose up --build
```

That starts PostgreSQL, applies the migrations, loads 20,000 generated events across nine months with `COPY`, and serves the API on port 8081. Reading needs the admin token, and sending needs the ingest token.

```sh
T='Authorization: Bearer admin-token'

# Send an event. The persons arrive inside it, and are stored apart.
curl -X POST localhost:8081/events -H 'Authorization: Bearer ingest-token' -d '{
  "service": "crm", "action": "customer.viewed",
  "actor": {"id": "mallory", "name": "Mallory"},
  "summary": "Mallory opened a customer record",
  "metadata": {"region": "eu"},
  "persons": [
    {"ref": "p-1", "name": "Ann", "role": "viewed", "fields": ["email", "phone"]},
    {"ref": "p-2", "role": "affected"}
  ]
}'

curl -H "$T" 'localhost:8081/events?person=p-1&meta.region=eu'   # who touched this person, in the EU?
curl -H "$T" 'localhost:8081/persons/p-1/timeline'               # everything that happened to them
curl -H "$T" 'localhost:8081/stats/anomalies?persons=100'        # who touched too many people?
curl -H "$T" 'localhost:8081/admin/partitions'                   # the months on disk
curl -H "$T" -X POST 'localhost:8081/admin/cleanup'              # drop expired months now
```

## Routes

| Route | Does | ant features |
| --- | --- | --- |
| `POST /events`, `POST /events/batch` | Stores events, safe to retry | Graph `Insert` with batching, validators, `Tx`, `InsertMany` with conflict arithmetic |
| `GET /events` | Searches, a page at a time | `When`, `Has`, `After` cursors, `Include`, `With`, `CountRelated`, JSON and full-text conditions |
| `GET /events/{id}` | Reads one event | `Include` across partitions |
| `GET /events/{id}/chain` | Follows what an event caused | `Recursive` CTE |
| `GET /export` | Streams NDJSON | `Each`, row by row |
| `GET /persons/{ref}` | Summarizes a person | A view, `Project`, `GroupBy`, `CountDistinct` |
| `GET /persons/{ref}/timeline` | Lists what happened to them | `JoinOn` on both key columns, `RowNumber`, `Lag` |
| `POST /persons/{ref}/erase` | Removes their identity | `UpdateReturning` in a `Tx`, across every month |
| `GET /stats/activity` | Counts by hour, day, week, or month | `GroupByExpr`, `Filter` on aggregates, `CountDistinct` |
| `GET /stats/rollup` | Reads running totals | A window over an aggregate through `ant.SQL` |
| `GET /stats/top-actors` | Ranks actors | `Rank` over grouped rows |
| `GET /stats/anomalies` | Finds odd behaviour | `Having`, `Exists`, `NotExists` |
| `GET /stats/latest` | Newest event per actor | `JoinLateral` with `Limit` |
| `GET /admin/partitions` | Lists months and sizes | `ant.Raw` into a plain struct |
| `POST /admin/cleanup` | Runs retention now | `postgres.MonthlyPartitions`, `UseConn`, advisory lock |
| `GET /metrics` | Prometheus text | `metrics.New` |

## How the data is stored

Two tables are partitioned by month of `occurred_at`:

```text
events                     event_persons
├── events_2026_09         ├── event_persons_2026_09
├── events_2026_10         ├── event_persons_2026_10
└── events_2026_11         └── event_persons_2026_11
```

The API takes persons inside the event, but they live in their own table, because the question this service exists to answer is "what happened to this person?", and that has to be one indexed lookup across months.

Partitioning shapes the model in three ways, all visible in [models/event.go](models/event.go) and [models/person.go](models/person.go):

**The key includes the time.** PostgreSQL can only enforce uniqueness per partition, so a partitioned table's keys must contain the partition column: `CompositeKey(ID, OccurredAt)`.

**A foreign key needs both columns.** The constraint goes from `(event_id, occurred_at)` to `(id, occurred_at)`, which a relation cannot express. So the relations say how to load rows, with `.NoForeignKey()`, and `b.ForeignKey(...)` declares the constraint on its own.

**Each person repeats the event's time**, which is what puts the two rows in matching months. Joins name both columns so that PostgreSQL pairs each month with its own, instead of searching all of them.

Migrations create the parent tables. Partitions are not schema: they come and go on a schedule, `drift` ignores them, and [services/retention.go](services/retention.go) manages them with `postgres.MonthlyPartitions`.

## Retention

| Variable | Default | Purpose |
| --- | --- | --- |
| `RETENTION_MONTHS` | 12 | Whole months to keep besides the current one |
| `PARTITIONS_AHEAD` | 3 | Future months to keep ready |
| `CLEANUP_CRON` | none | When to run, as `0 3 * * *`. Empty means only `POST /admin/cleanup` |

A run creates upcoming months and drops expired ones. Dropping a partition takes a moment and frees its space at once, where deleting a month of rows would take hours and leave the table bloated. Only whole months go, so nothing newer than the cutoff can be lost.

Every instance schedules this, and a PostgreSQL advisory lock lets one of them work. The lock belongs to a session, which is why `Run` holds one connection through `ant.UseConn`. `event_persons` goes before `events`, because a partition others refer to cannot be dropped.

Actors and daily counts are not partitioned, so totals outlive the rows behind them.

## Other variables

| Variable | Required | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `INGEST_TOKEN` | yes | Lets systems send events |
| `ADMIN_TOKEN` | yes | Lets people read and manage them |
| `PORT` | no | Defaults to 8080 |
| `LOG_LEVEL` | no | `debug` logs every statement |

Two tokens, because the systems that write an audit log should not be able to read it.

## Writing many events

Ingest uses `db.Events.Insert`, which walks each event's persons and writes each table in batches of 200, so a thousand events cost a few statements, not a few thousand. Everything, including the running totals, is one transaction.

A sender that retries after a timeout sends the same IDs again. IDs already stored are counted as duplicates and skipped, so the call is safe to repeat. IDs are UUIDv7 from `ant/ids`, which sort by time and keep inserts at the end of the index.

For a backfill, [cmd/loadgen](cmd/loadgen/main.go) uses `pgxext.Copy`, which loads about 20,000 rows a second here, indexes included.

## Good to know

**A bound value in a grouped expression fails.** PostgreSQL sees `date_trunc($1, x)` in `SELECT` and `date_trunc($2, x)` in `GROUP BY` as different expressions. [services/stats.go](services/stats.go) writes the unit into the SQL from a fixed list.

**A row outside the partitions is an error**, reported as a 400. A default partition would hide the problem, then block creating that month later.

**Filter by time whenever you can.** PostgreSQL skips partitions that cannot match, so a time range is the cheapest condition there is.

## Tests

```sh
docker compose up -d db
TEST_DATABASE_URL='postgres://audit:audit@localhost:55441/audit?sslmode=disable' go test ./...
```

They build a schema of their own from the real migrations, and cover duplicates, invalid batches, cursors across months, erasure, and two instances running retention at once.
