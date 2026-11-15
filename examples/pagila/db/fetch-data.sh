#!/bin/sh
# Downloads the Pagila data (3 MB) next to this script, pinned to the tag the
# schema in this directory comes from, and checks it.
set -eu

cd "$(dirname "$0")"

TAG=pagila-v3.1.0
SUM=fb81bec377687c83e11d2a24916ae28656d85550bf0ada798305bf7e2af9823b

if [ ! -f pagila-data.sql ]; then
	curl -sSfL -o pagila-data.sql "https://raw.githubusercontent.com/devrimgunduz/pagila/$TAG/pagila-data.sql"
fi

echo "$SUM  pagila-data.sql" | sha256sum -c -
