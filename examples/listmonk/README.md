# listmonk

A slice of a real application's data layer, ported to ant. [listmonk](https://github.com/knadh/listmonk) is a popular newsletter manager in Go on PostgreSQL: a schema with a dozen enum types, JSONB attributes, arrays, a link table with a composite key, materialized views for the dashboards, and about a hundred hand-written queries in one `queries.sql`. This example keeps listmonk's schema and data exactly as its own installer makes them, describes the tables as ant models, and ports the queries that matter most to ant, each one checked against listmonk's original SQL on the same rows.

It is not a fork of listmonk. It is what a team would have after the first weeks of moving a real data layer to ant, with the parts that did not fit written down.

## License

listmonk is AGPLv3. None of its Go source is copied here. The models, the queries and the API are written from scratch against the schema. listmonk's `schema.sql` and `queries.sql` are not in this repository either: `scripts/fetch-upstream.sh` downloads them at a pinned tag (`v5.0.0`) into `upstream/`, which git ignores, and the tests read the original queries from there to compare results.

## Run it

You need PostgreSQL and Docker. Create an empty database, then let listmonk's own installer fill it:

```sh
createdb listmonk
make install DB_USER=listmonk DB_PASSWORD=listmonk DB_NAME=listmonk   # runs listmonk/listmonk:v5.0.0 --install
make test   DB_USER=listmonk DB_PASSWORD=listmonk DB_NAME=listmonk   # fetches upstream/, then go test ./...
make drift  DB_USER=listmonk DB_PASSWORD=listmonk DB_NAME=listmonk   # the models against the live schema
make run    DB_USER=listmonk DB_PASSWORD=listmonk DB_NAME=listmonk   # the API on localhost:9100
```

Without `TEST_DATABASE_URL` the tests print a note and pass, so `go test ./...` alone proves nothing. Each test runs in a transaction that rolls back, so the database stays as the installer left it.

```sh
curl 'localhost:9100/api/subscribers?list_id=1&order_by=email&order=asc&per_page=10'
curl "localhost:9100/api/subscribers?query=subscribers.attribs->>'city'%3D'Bengaluru'"
curl -X POST localhost:9100/api/subscribers -d '{"email":"ada@example.org","name":"Ada","lists":[1,2],"preconfirm_subscriptions":true}'
curl -X PUT localhost:9100/api/subscribers/3 -d '{"name":"Ada Lovelace","lists":[2]}'
curl 'localhost:9100/api/lists?order_by=subscriber_count&order=desc'
curl -X PUT localhost:9100/api/campaigns/1/status -d '{"status":"running"}'
```

## Layout

| Package | What it is |
| --- | --- |
| [models](models/) | One struct per table and view, written by hand from `schema.sql`. `migrate drift` reports no difference |
| [core](core/) | The data layer, shaped like listmonk's `internal/core`: one method per operation. A `Core` wraps one handle: the root, a request's session, or a transaction, so several methods can share one transaction with `c.With(tx)` |
| [api](api/) | A slice of the admin API on `net/http`. Each request gets its own session. Responses are DTOs in listmonk's shape |
| [cmd/migrate](cmd/migrate/) | ant's migration tool, for `drift`, `check` and `diff` |

## What was ported, and how it went

Each row is a query in listmonk's `queries.sql`. The tests run the port and the original on the same data and compare the rows, or the state the statement leaves behind.

| listmonk query | Port | How |
| --- | --- | --- |
| `get-subscriber` | typed | `Where` on ID, UUID or e-mail, `Include` of the subscriptions and their lists |
| `insert-subscriber` | typed, longer | One CTE there; here a transaction: `InsertOnly`, then `InsertMany` of the subscriptions with `OnConflict(...).And(...).DoUpdate(...)` on the composite key |
| `upsert-subscriber` | typed, longer | `Upsert` on the e-mail, which returns the row's ID and status, then the subscriptions as above. `Upsert` runs no insert hooks, so the UUID is set by hand |
| `update-subscriber-with-lists` | change tracking | Load the subscriber with its subscriptions, edit the struct and the slice, `SaveChanges`. Removing an element deletes that subscription |
| `query-subscribers` + `-count` | typed, with one fragment per part | Optional filters with `When`, the list filter as `Has`, the search as `SQLCond("? ~* ?")`, the admin's own SQL condition as `SQLCond(expr)` in a read-only transaction, sort from a whitelist map |
| `blocklist-subscribers(-by-query)` | typed | Two bulk `Update`s; by query, `InSub` over the same filter |
| `delete-subscribers-by-query`, `delete-orphan-subscribers` | typed | `InSub`; `HasNone` |
| `add-subscribers-to-lists-by-query` | typed, awkward | `InsertFrom` over `CrossJoin`. `InsertFrom` takes no conflict clause, so existing pairs are left out with `NotExists`, which a concurrent insert can still race |
| `query-campaigns` | typed | The lists come back as a loaded relation instead of a JSON aggregate. Array containment and full-text search are fragments |
| `create-campaign` | typed, longer | One CTE there, five statements here: default template, audience count, insert with the list rows as a relation |
| `update-campaign-status` | typed, awkward | `UpdateReturning` with a `CASE`; each enum value needs a `::campaign_status` cast by hand |
| `next-campaigns` | typed, longer | `ForUpdate().SkipLocked()` with `Include`, the audience counted per campaign, the changes saved by tracking |
| `get-running-campaign` + `next-campaign-subscribers` | typed | Keyset batches: `Gt` on the checkpoint, the audience as two `Has` conditions split by the lists' opt-in, `Limit`, then the checkpoint saved |
| `query-lists` | typed + one read | Counts from the materialized view as a correlated subquery, sortable by `subscriber_count`; the per-status counts in a second query |
| `get-settings`, `update-settings` | typed | Rows into a map; one `Update` per key, since the table has no primary key |
| `create-link`, `register-link-click` | typed | An upsert that returns the existing UUID; three reads and an insert instead of one `INSERT` with subqueries |
| `record-bounce` | typed, longer | Five CTEs there; a transaction with a row lock here |
| `query-bounces` | typed | A join and a left join, projected into a row type |
| dashboard counts, `REFRESH MATERIALIZED VIEW` | view model + raw | The views are models with `b.View`; refreshing one is `ant.Raw(...).Exec` |
| views per day with empty days | raw | `generate_series` has no typed form. A typed version without the empty days is beside it |

## What stayed raw SQL, and why

- **The admin's segmentation query.** listmonk lets an admin type a SQL condition, such as `subscribers.attribs->>'city' = 'Riga'`. That is SQL by design, so it is `ant.SQLCond(expr)` with no arguments, run in a read-only transaction. A condition with a `?` in it, such as the `jsonb ?` operator, works only while it has no arguments of its own.
- **Materialized views.** ant reads them as view models but has no statement to refresh them.
- **`generate_series`.** A series of days to join against has no typed source.
- **Fragments inside typed queries:** regular expressions (`~*`), array containment (`@>`, `<@`), full-text search with the default configuration, `COALESCE` around `MAX` and `SUM`, and enum casts inside `CASE`.

## Migrations

The models describe the schema listmonk made, and `drift` reports nothing. listmonk keeps its schema and its own history (the `migrations` key in `settings`, run by `listmonk --upgrade`), so this example uses ant the way its guide calls [keeping your current runner](https://skudra.net/migrations/adopting#keep-your-current-runner): `snapshot` records the models, and `check` and `drift` keep them honest. ant's `ant_migrations` table and listmonk's history do not collide, and `listmonk --upgrade` runs normally on a database ant has baselined. One of them must own each change, though: a column listmonk adds in a release would also show up in ant's next `diff`.

What drift does not see: the materialized views, the `pgcrypto` extension, and the columns of a view model. Serial key columns compare equal to ant's identity keys.

## Limits hit

The port found these, and the tests show the workarounds:

- A tracked row read again with `ForUpdate` comes back with the values the session already had, not the locked row's.
- A session of a root opened with `WithTracking(false)` has tracking off too, so `SaveChanges` there saves nothing and returns `nil`. The API turns it on per request.
- `ant.ReadOnly()` on a `Tx` inside another `Tx` is ignored, so the read-only guard on the admin's SQL does not hold inside a caller's transaction, or under `anttest.Isolate`.
- Entities loaded inside a nested `Tx` stay tracked after the outer transaction rolls back. With `anttest.Isolate` on one shared tracking root, a later test's `SaveChanges` wrote an earlier test's rows. The tests take a session per test.
- `ant.Max` and `ant.Sum` are typed as never `NULL`, but are `NULL` over no rows.
- A bound value inside `CASE` is text to PostgreSQL, so writing a `CASE` into an enum column fails without a cast.
- `postgres.Array` reads a `NULL` element as the string `"NULL"`.
- A field named `IDs` or `URLs` maps to the column `i_ds` or `ur_ls`, so `Raw` leaves it empty.
- `Lt`, `Gt` and `OrderBy` take no nullable field; `ant.Col(&C.SendAt).Lte(&now)` and `Order(ant.Col(...).Asc())` do.
- Times come back in the server's local zone, so the DTOs convert them to UTC before they reach JSON.
