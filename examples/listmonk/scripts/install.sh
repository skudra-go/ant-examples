#!/bin/sh
# Creates listmonk's schema and sample data in the database named by
# DATABASE_URL's parts, with listmonk's own installer, at a pinned version.
# The data is what a real listmonk install starts with: two lists, two
# subscribers, a campaign, templates, settings and an admin user.
set -eu

TAG="${LISTMONK_TAG:-v5.0.0}"

docker run --rm --network host \
	-e LISTMONK_db__host="${DB_HOST:-127.0.0.1}" \
	-e LISTMONK_db__port="${DB_PORT:-5432}" \
	-e LISTMONK_db__user="${DB_USER:-listmonk}" \
	-e LISTMONK_db__password="${DB_PASSWORD:-listmonk}" \
	-e LISTMONK_db__database="${DB_NAME:-listmonk}" \
	-e LISTMONK_db__ssl_mode=disable \
	-e LISTMONK_app__address=localhost:9000 \
	-e LISTMONK_ADMIN_USER=admin \
	-e LISTMONK_ADMIN_PASSWORD=listmonk-admin \
	"listmonk/listmonk:${TAG}" \
	./listmonk --install --idempotent --yes
