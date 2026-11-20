# AGENTS.md

Guide for coding agents, and for people, working on ant-examples: runnable projects that show how the [ant](https://github.com/skudra-go/ant) ORM is used in practice. Each example is its own Go module. `README.md` says what each one shows. The examples with an `agents.md` of their own (blog, audit, ledger) have rules for that project: read it before working there.

## How each example gets ant

| Example | ant | Needs |
|---|---|---|
| `examples/hello` | a release, from the module proxy | Go |
| `examples/pagila` | a release, from the module proxy | PostgreSQL with the Pagila database (`make db`) |
| `examples/blog`, `examples/audit`, `examples/ledger` | `replace github.com/skudra-go/ant => ../../../ant`, an ant checkout next to this repository | Docker |
| `examples/listmonk` | the same `replace` | PostgreSQL with listmonk's schema (`make install`), and `make upstream` for its SQL |

- An example on a release shows what users get. Move it to a newer release only once its code builds against it, and run `go mod tidy` in it.
- An example on the `replace` builds against whatever the ant checkout holds, so a breaking change in ant's `main` breaks it at once. After such a change lands, update the example in the same week, or say in its README which ant it needs.
- Docker builds use the folder holding both repositories as their context (`context: ../../..` in `docker-compose.yml`). `go build` refuses a `go.mod` or `go.sum` that needs updating, so a dependency change in ant needs `go mod tidy` in each example that replaces it.

## Checks

```sh
make fmt     # needs the ant checkout next to this repository, for antfmt
make lint    # antfmt -l, go vet, and golangci-lint in every example
```

Each example's own tests, from its folder:

```sh
go test ./...                                    # hello
TEST_DATABASE_URL=... go test ./...              # blog, audit, ledger (see each agents.md)
make test                                        # pagila, listmonk
```

`make lint` must pass before you finish. Database tests need their server; say which ran.

## Rules

- Write code the way ant does: one field or argument per line when a call wraps, and blank lines around blocks and before a `return`. `make fmt` enforces it.
- The examples teach, so they use ant the way its docs recommend: a session per request for `SaveChanges`, models through probes (`&models.Users.Email`), schema changes through `migrate diff`, and no down migrations.
- `examples/listmonk` ports listmonk, which is AGPL: its schema and SQL come from `scripts/fetch-upstream.sh` at a pinned tag, and no upstream Go code is copied in.
- `examples/pagila` adopts an existing database: its models are written by hand and checked with `drift`, never generated from the database.
- If an example shows a bug in ant, open the issue in the ant repository, with the smallest code that shows it.

## Branches

GitHub Flow on `main`: a branch per change, rebased on `main`, and a pull request with a Conventional Commits title (`fix:`, `feat:`, `docs:`).
