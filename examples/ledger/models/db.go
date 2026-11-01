// Package models holds the database's structs and the context that ties them
// together. It imports nothing from this project.
package models

import "github.com/skudra-go/ant"

// AppDB is the database.
type AppDB struct {
	ant.Context
	Tenants   ant.Table[Tenant]
	Accounts  ant.Table[Account]
	Entries   ant.Table[Entry]
	Lines     ant.Table[Line]
	Outbox    ant.Table[Message]
	Statement ant.Table[StatementLine]
}
