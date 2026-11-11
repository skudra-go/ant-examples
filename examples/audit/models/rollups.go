package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Actors is the probe for Actor.
var Actors = ant.Fields[Actor]()

// Actor is who does things, kept up to date as events arrive. It outlives the
// partitions that hold its events.
type Actor struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
	EventCount int64     `json:"eventCount"`
}

// DailyCounts is the probe for DailyCount.
var DailyCounts = ant.Fields[DailyCount]()

// DailyCount is a running total, kept on ingest so that a year of statistics reads
// a few thousand rows and not a hundred million. It outlives the partitions too.
type DailyCount struct {
	Day     time.Time `json:"day"`
	Service string    `json:"service"`
	Action  string    `json:"action"`
	Outcome string    `json:"outcome"`
	Count   int64     `json:"count"`
}

// Configure says what the struct cannot.
func (DailyCount) Configure(b *ant.ModelBuilder[DailyCount]) {
	b.CompositeKey(
		ant.F(&DailyCounts.Day),
		ant.F(&DailyCounts.Service),
		ant.F(&DailyCounts.Action),
		ant.F(&DailyCounts.Outcome),
	)

	b.Column(&DailyCounts.Day).Type("date")
	b.Column(&DailyCounts.Outcome).Type("event_outcome")
}

// PersonOverviews is the probe for PersonOverview.
var PersonOverviews = ant.Fields[PersonOverview]()

// PersonOverview is a row of the person_overviews view, defined in
// migrations/code/10_person_overviews.sql.
type PersonOverview struct {
	Ref       string    `json:"ref"`
	Events    int64     `json:"events"`
	Actors    int64     `json:"actors"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// Configure says what the struct cannot.
func (PersonOverview) Configure(b *ant.ModelBuilder[PersonOverview]) {
	b.View("person_overviews")
}
