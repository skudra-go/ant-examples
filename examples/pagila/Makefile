# DATABASE_URL points at the Pagila database that db/load.sh creates.
DATABASE_URL ?= postgres://ant:ant@127.0.0.1:55432/dx_pagila?sslmode=disable
export DATABASE_URL

.PHONY: db adopt run test drift

# db creates dx_pagila from the upstream Pagila schema and data, as the team's
# existing database.
db:
	./db/load.sh

# adopt records the existing schema as applied, then applies what ant added since.
adopt:
	go run ./cmd/migrate baseline 0001
	go run ./cmd/migrate up

run:
	go run ./cmd/pagila

test:
	go test -count=1 ./...

drift:
	go run ./cmd/migrate drift
