# Blog

A JSON API for a blog, started from nothing. It has users, posts, comments, and tags, and shows the parts of [ant](https://github.com/skudra-go/ant) a real project touches: relations, change tracking, soft deletes, and above all migrations, from the first table to a rename on a table with data in it.

## Run it

```sh
docker compose up --build
```

That starts PostgreSQL, applies the migrations, adds demo data, and serves the API on port 8080.

```sh
curl localhost:8080/posts
curl localhost:8080/posts/hello-ant
curl localhost:8080/stats

curl -X POST localhost:8080/posts -H 'Authorization: Bearer dev-token' \
  -d '{"authorId":1,"title":"My first post","body":"Hello.","tags":["go"]}'
curl -X POST localhost:8080/posts/my-first-post/publish -H 'Authorization: Bearer dev-token'
```

| Route | Does | Token |
| --- | --- | --- |
| `GET /healthz` | Checks the database | |
| `GET /posts?tag=&page=&size=` | Lists published posts | |
| `GET /posts/{slug}` | Reads a post and its comments | |
| `GET /stats` | Reads the `post_stats` view | |
| `GET /users/{id}` | Reads a user | |
| `POST /posts/{slug}/comments` | Adds a comment | |
| `POST /posts` | Creates a draft | yes |
| `PATCH /posts/{slug}` | Edits a post | yes |
| `POST /posts/{slug}/publish` | Publishes a draft | yes |
| `DELETE /posts/{slug}` | Soft deletes a post | yes |
| `POST /users` | Creates a user | yes |

## Layout

```text
.
├── app.go, config.go, services.go   the App, the environment, the services
├── cmd/
│   ├── migrate/                     the schema commands
│   └── server/                      the API
├── migrations/
│   ├── versions/                    SQL that runs once, and the snapshot
│   ├── code/                        SQL that reruns when it changes
│   ├── data.go                      data migrations in Go
│   ├── seeds.go                     demo data
│   └── migrations.go                embeds the SQL
├── models/                          the database's structs, one file per model
├── services/                        business logic, no HTTP
├── routes/                          HTTP, no business logic
│   ├── requests/                    what the API accepts
│   └── responses/                   what the API returns
├── middleware/                      what wraps every request
└── utils/                           small helpers
```

Dependencies point one way: `routes` to `services` to `models`, and `cmd` at the top. These choices hold up as a project grows.

**`models` is at the top level, not under `routes`.** Migrations, seeds, and services all need the structs, and `cmd/migrate` should build without the web server. It imports nothing from this project, so nothing can pull HTTP into it.

**All models live in one package.** `User.Posts` and `Post.Author` refer to each other, and Go forbids import cycles, so a package per model cannot work. A file per model keeps it readable at any size, with the model, its probe, and its `Configure` together.

**`requests` and `responses` stay separate from models.** A request lists what a client may set, so `authorId` can never overwrite an `id`. A response embeds the model and adds what a client needs, so a new column appears by itself, and a field tagged `json:"-"`, such as a relation, stays out:

```go
type Post struct {
    models.Post
    Author *Author  `json:"author,omitempty"`
    Tags   []string `json:"tags"`
}
```

**Migrations are a package.** Go data migrations need a home that both binaries import, and `go:embed` must sit next to the files. It imports `models`, and nothing imports it except `app.go` and `cmd`.

**Probes are named after their table.** `models.Posts.Slug` reads like a column, and unlike `P`, the names do not run out at the fifth model.

**The shared context does not track.** A tracker keeps everything it loads, so on a long-lived context it grows without end and mixes up requests. `app.go` turns tracking off, and a service that wants `SaveChanges` opens a session for that one request, as `Posts.Update` does.

## How the schema got here

Every file in `migrations/versions` was written by `go run ./cmd/migrate diff`, in order, by changing the structs. Read them as a tour.

| File | What happened |
| --- | --- |
| `0001_create_blog` | The first `diff`, from the structs alone. Note the indexes on foreign keys, and the join table. |
| `0002_add_post_slug` | `slug` is required and unique. The generated file gave every existing post the same empty slug, and warned that this would fail, which it did, safely. So it was edited by hand to fill slugs from titles. |
| `0003_lowercase_tags` | In [migrations/data.go](migrations/data.go). A data fix, in Go, in sequence with the SQL. |
| `0004_rename_bio_to_about` | A rename, with `--rename users.bio=about`, which keeps the data. |
| `0005_index_published_posts` | A partial index, from one line in `Post.Configure`. |
| `code/10_post_stats.sql` | A view, which reruns when edited. `models.PostStat` reads from it. |

## Changing the schema

You need Go 1.27 or later, and the database running.

```sh
docker compose up -d db
export DATABASE_URL='postgres://blog:blog@localhost:55440/blog?sslmode=disable'
```

Change a struct, then:

```sh
go run ./cmd/migrate diff add_post_summary   # compare the structs with the snapshot, write SQL
go run ./cmd/migrate up                      # after reading the file
```

Other commands you will want:

```sh
go run ./cmd/migrate status   # what has run
go run ./cmd/migrate check    # did someone change a struct without a migration? For CI, no database needed
go run ./cmd/migrate drift    # does the database still match?
go run ./cmd/migrate help     # everything else
```

## Deployment

The image holds two binaries with the migrations embedded in both, and no other files.

The compose file runs `migrate up` as a step of its own before the server starts, so the server never runs on a schema older than its code, and a failed migration stops the rollout before anything changes. The `migrate` service would be a Kubernetes job or a release step elsewhere.

For a small deployment, set `AUTO_MIGRATE=true`, and the server applies migrations as it starts. Several instances can do this at once: one works, and the rest wait.

| Variable | Required | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `ADMIN_TOKEN` | yes | Guards the routes that write |
| `PORT` | no | Defaults to 8080 |
| `AUTO_MIGRATE` | no | Applies migrations when the server starts |
| `LOG_LEVEL` | no | `debug` logs every statement |

## Tests

```sh
go test ./...
```

The service tests skip without a database. With one, they build a schema of their own from the real migrations, so a broken migration fails there first:

```sh
TEST_DATABASE_URL="$DATABASE_URL" go test ./...
```
