# DB_* name the database; TEST_DATABASE_URL must point at the same one.
DB_HOST ?= 127.0.0.1
DB_PORT ?= 5432
DB_USER ?= listmonk
DB_PASSWORD ?= listmonk
DB_NAME ?= listmonk
TEST_DATABASE_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

.PHONY: upstream install test drift run

# listmonk's schema.sql and queries.sql at the pinned tag, for the tests.
upstream:
	./scripts/fetch-upstream.sh

# listmonk's own installer: schema, settings, and its sample data.
install:
	DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) DB_NAME=$(DB_NAME) ./scripts/install.sh

test: upstream
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test ./...

# The models against the live schema.
drift:
	DATABASE_URL='$(TEST_DATABASE_URL)' go run ./cmd/migrate drift

run:
	DATABASE_URL='$(TEST_DATABASE_URL)' go run ./cmd/server
