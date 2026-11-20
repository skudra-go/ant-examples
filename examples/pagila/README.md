# Pagila

ant adopted on a database that already exists. [Pagila](https://github.com/devrimgunduz/pagila) is the PostgreSQL port of MySQL's Sakila sample: a DVD rental chain with films, actors, stores, customers, rentals, and payments. It stands in for the database a team already runs, with everything such a database collects over the years: an enum, a domain, an array column, a `tsvector` kept by a trigger, `numeric` money, a payment table partitioned by month, `last_update` columns set by triggers, functions, views, a materialized view, `bytea`, composite keys on the link tables, singular table names, and sequences named after their tables.

The models were written by hand from the schema, the way the [adoption guide](https://skudra.net/migrations/adopting) says, and `drift` checked them until nothing differed. Then `adopt` and `baseline` started the history without touching the data, and the first migration after that adds a column, an index, and a table.

## Run it

You need PostgreSQL 12 or later. The scripts assume the `ant-postgres-1` container from the ant repository (`make db-up` there), and take `PSQL` and `DB` to use another server.

```sh
make db      # creates dx_pagila from the upstream schema and data
make adopt   # baseline 0001, then up applies 0002
make run     # the queries below
make test    # each query checked against plain SQL
```

`DATABASE_URL` defaults to `postgres://ant:ant@127.0.0.1:55432/dx_pagila?sslmode=disable`.

```text
Documentaries: 68. The first, ACADEMY DINOSAUR, stars JOHNNY CAGE, ROCK DUKAKIS, ...
Top customers:
  KARL       SEAL          45 payments   221.55
Revenue by store and month (payment is partitioned by month):
  store 1 2022-01   353 payments    1547.48
Overdue on 2022-09-01: 183 rentals, the oldest PEACH INNOCENT, due 2022-02-17
Full-text search for "astronaut boat": 12 films, first CHRISTMAS MOONSHINE
Customer 1: MARY SMITH, 1913 Hanoi Way, Sasebo, Japan
```

## What is where

| Path | Holds |
| --- | --- |
| `db/pagila-schema.sql` | The upstream schema, unchanged |
| `db/fetch-data.sh` | Downloads the upstream data (3 MB) and checks its hash |
| `db/load.sh` | Creates the database and loads both |
| `models/` | One struct per table, two views, and `AppDB` |
| `migrations/versions/0001_baseline.up.sql` | The schema as it was on the day of adoption, from `pg_dump` |
| `migrations/versions/0002_add_imdb_and_reviews.up.sql` | The first change made through ant |
| `services/` | The queries, and tests that compare each with plain SQL |
| `cmd/migrate`, `cmd/pagila` | The migration tool and the demo |

## How it was adopted

1. Write the models from `db/pagila-schema.sql`. Each table needs `b.Table` for its singular name and `b.Key` for its `<table>_id` key. `last_update` is mapped to `UpdatedAt`, so ant moves it too.
2. Run `go run ./cmd/migrate drift` until it prints `No drift`. The first run found five things: the two link tables needed their foreign keys declared as relations, and `language.name` is `character(20)`, not `text`.
3. Dump the schema without owners and grants, adopt it, and mark the existing database:

   ```sh
   pg_dump --schema-only --no-owner --no-privileges "$DATABASE_URL" > schema.sql
   go run ./cmd/migrate adopt schema.sql
   go run ./cmd/migrate baseline 0001
   ```

   Without `--no-owner`, the baseline carries `ALTER ... OWNER TO` for the role that ran `pg_dump`, and a new database created by any other role fails on it.
4. Change the models and run `diff`. `0002` holds only the new `imdb_id` column, its partial unique index, and the `film_review` table with its foreign keys to `film` and `customer`.

Give `baseline` the version. With no version, ant v0.3.2 records every migration as applied, so a database adopted after `0002` was written would skip it without a word, and only `drift` would notice. Later versions of ant refuse when more than one migration is left to record.

## The queries

| Question | ant |
| --- | --- |
| Films in a category, with their actors | `Has` through `film_category`, `Include` through `film_actor` |
| Top customers by spending | `Join`, `GroupBy` with `Then`, `Project`, `sum` through `ant.SQL` |
| Revenue per store and month | `GroupByExpr` on `date_trunc`, over the partitioned table |
| Overdue rentals | Two joins, an `ant.SQL` due date in `Where` and `Order` |
| Copies in stock | `ant.Func` calling `inventory_in_stock()` in `Where` |
| `film_in_stock()`, `rewards_report()` | `ant.Raw` and `ant.FromFunc` on set-returning functions |
| Full-text search | `ant.SQLCond` with `@@` on the `tsvector` column |
| By rating and special feature | `Eq` on the enum, `ANY` on the array through `ant.SQLCond` |
| A customer's address, city, and country | Nested `IncludeOne` |
| Paging films | `Page` |
| Returning a rental | `Find`, change, `SaveChanges`, `Reload` |
| Taking a payment | `Insert` into the partitioned table, key read back from the sequence |
| Deleting a language in use | `ant.IsForeignKeyViolation` |

## Limits found

These are what didn't fit, and what the example does instead.

- **Columns that triggers set go stale.** ant reads nothing back after an `UPDATE`. After `SaveChanges`, the tracked film keeps the `last_update` ant wrote, which the trigger replaced, and the old `fulltext`. `ReturnRental` calls `Reload`. For the same reason `last_update` can't be a concurrency token: the second save of an untouched row reports a conflict.
- **`boolean DEFAULT true` turns `false` into `true`.** `drift` wants a default on `customer.activebool`, and a default makes a zero value mean "use it", so an inactive customer would be inserted active. Here the field keeps the tag, because Pagila never inserts inactive customers. To allow it, make the field `*bool` with `.NotNull()`.
- **`postgres.Match` can't search a `tsvector` column.** It wraps the column in `to_tsvector`, which takes text. The search uses `ant.SQLCond` with `@@`.
- **Money is `decimal.Decimal`, so typed aggregates are out.** `Sum`, `SumOf`, and `Avg` take only Go numbers, so sums go through `ant.SQL[decimal.Decimal]("sum(?)", ...)`.
- **The partitions keep their names.** Pagila's months are `payment_p2022_01`. `postgres.MonthlyPartitions` lists none of them, and `Ensure` fails on the overlap, so new months need SQL of your own.
- **Foreign keys live on Pagila's partitions, not the parent,** so the payment relations are declared with `NoForeignKey`.
- **Triggers, functions, domains, views, and the materialized view stay in the baseline.** The models name the `year` domain with `.Type("year")`, and `drift` accepts it, but a database built from the models alone would fail on it. That is why the history starts from the dump.
- **`get_customer_balance()` is broken upstream:** it calls MySQL's `IF`.

## The data

`db/pagila-schema.sql` and the data that `db/fetch-data.sh` downloads come from the `pagila-v3.1.0` tag. Newer versions need PostgreSQL 18 (`uuidv7()`, virtual generated columns) and the pgvector extension. That tag is published under its own MIT-style licence, copied in `db/LICENSE-pagila.txt`, which allows redistribution with the notice. Later versions moved to the PostgreSQL licence.

`db/load.sh` drops the dump's `OWNER TO postgres` lines, so any superuser can load it.
