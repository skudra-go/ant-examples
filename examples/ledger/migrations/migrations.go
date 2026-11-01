// Package migrations holds the database's history. The SQL is embedded, so the
// image needs no files beside its binaries.
package migrations

import "embed"

// Files holds the SQL. The all: prefix also takes files starting with a dot, so a
// folder holding only a .gitkeep still compiles, which a new project needs.
//
//go:embed all:versions all:code
var Files embed.FS
