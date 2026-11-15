#!/bin/sh
# Downloads listmonk's schema.sql and queries.sql at a pinned tag into
# ./upstream. listmonk is AGPLv3, so its files are not kept in this repository:
# the tests read queries.sql to compare the ant port with the original SQL.
set -eu

TAG="${LISTMONK_TAG:-v5.0.0}"
DIR="$(dirname "$0")/../upstream"
BASE="https://raw.githubusercontent.com/knadh/listmonk/${TAG}"

mkdir -p "$DIR"

for f in schema.sql queries.sql permissions.json; do
	curl -fsSL "${BASE}/${f}" -o "${DIR}/${f}"
done

echo "Fetched listmonk ${TAG} into ${DIR}"
