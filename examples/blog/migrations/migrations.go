// Package migrations holds the database's history: generated SQL in versions,
// views in code, and data migrations in Go. It is embedded in both binaries, so a
// deployment needs no files beside them.
package migrations

import "embed"

// Files holds the SQL. The all: prefix also takes files starting with a dot, so a
// folder holding only a .gitkeep still compiles, which a new project needs.
//
//go:embed all:versions all:code
var Files embed.FS
