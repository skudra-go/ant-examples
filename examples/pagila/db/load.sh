#!/bin/sh
# Creates the database and loads the upstream Pagila schema and data into it,
# the way a team's existing database would look before it meets ant.
#
#   PSQL     how to run psql (default: psql inside the ant-postgres-1 container)
#   DB       the database to create (default: dx_pagila)
set -eu

cd "$(dirname "$0")"

PSQL=${PSQL:-docker exec -i ant-postgres-1 psql -U ant -v ON_ERROR_STOP=1 -q}
DB=${DB:-dx_pagila}

./fetch-data.sh

$PSQL -d postgres -c "DROP DATABASE IF EXISTS $DB WITH (FORCE)"
$PSQL -d postgres -c "CREATE DATABASE $DB"

# The dump assigns every object to a role named postgres. Leave ownership to
# whoever loads it, so the script works with any superuser name.
grep -v 'OWNER TO postgres;' pagila-schema.sql | $PSQL -d "$DB"
$PSQL -d "$DB" < pagila-data.sql > /dev/null
$PSQL -d "$DB" -c 'REFRESH MATERIALIZED VIEW rental_by_category'
