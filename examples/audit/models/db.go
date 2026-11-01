// Package models holds the database's structs and the context that ties them
// together. It imports nothing from this project, so the migrate command builds
// without the web server.
package models

import "github.com/skudra-go/ant"

// AppDB is the database.
type AppDB struct {
	ant.Context
	Events      ant.Table[Event]
	Persons     ant.Table[Person]
	Actors      ant.Table[Actor]
	DailyCounts ant.Table[DailyCount]

	// PersonOverviews is a view, defined in migrations/code.
	PersonOverviews ant.Table[PersonOverview]
}
