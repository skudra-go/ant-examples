package models

import (
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"
	"github.com/skudra-go/ant/validate"
)

// Events is the probe for Event.
var Events = ant.Fields[Event]()

// Outcome values.
const (
	OutcomeSuccess = "success"
	OutcomeDenied  = "denied"
	OutcomeFailure = "failure"
)

// Event is one thing that happened in some system. Its table is split by month,
// which is why its key includes OccurredAt: PostgreSQL requires a partitioned
// table's unique keys to contain the partition column. A key of the ID alone could
// not be checked across months.
type Event struct {
	ID           string                   `json:"id"`
	OccurredAt   time.Time                `json:"occurredAt"`
	CreatedAt    time.Time                `json:"receivedAt"`
	Service      string                   `json:"service"`
	Action       string                   `json:"action"`
	Outcome      string                   `json:"outcome"`
	ActorID      string                   `json:"actorId"`
	ActorName    string                   `json:"actorName"`
	ResourceType string                   `json:"resourceType,omitempty"`
	ResourceID   string                   `json:"resourceId,omitempty"`
	IP           *string                  `json:"ip,omitempty"`
	ParentID     *string                  `json:"parentId,omitempty"`
	Summary      string                   `json:"summary"`
	Metadata     ant.JSON[map[string]any] `json:"metadata"`

	// Search is kept by the database, for full-text queries. It is never read or
	// written from Go.
	Search string `json:"-" ant:"writeonly"`

	// PersonCount has no column. Queries fill it with ant.CountRelated.
	PersonCount int64 `json:"personCount" ant:"computed"`

	Persons  []Person `json:"-"`
	Children []Event  `json:"-"`
}

// Configure says what the struct cannot.
func (Event) Configure(b *ant.ModelBuilder[Event]) {
	b.Comment("Audit events, partitioned by month of occurred_at")
	ant.GenerateKey(b, &Events.ID, ids.UUID)
	b.CompositeKey(ant.F(&Events.ID), ant.F(&Events.OccurredAt))
	b.PartitionByRange(ant.F(&Events.OccurredAt))

	b.Column(&Events.ID).Type("uuid")
	b.Column(&Events.ParentID).Type("uuid")
	b.Column(&Events.Outcome).Enum("event_outcome", OutcomeSuccess, OutcomeDenied, OutcomeFailure)
	b.Column(&Events.IP).Type("inet")
	b.Column(&Events.Search).Type("tsvector").Generated("to_tsvector('simple', summary)")

	b.Index(&Events.Service).And(&Events.OccurredAt).Desc()
	b.Index(&Events.ActorID).And(&Events.OccurredAt).Desc()

	b.Index(&Events.ResourceType).
		And(&Events.ResourceID).
		Where("resource_id <> ''").
		Name("events_resource_idx")

	b.Index(&Events.Metadata).Using("gin").Name("events_metadata_idx")
	b.Index(&Events.Search).Using("gin").Name("events_search_idx")
	b.Index(&Events.ParentID).Where("parent_id IS NOT NULL").Name("events_parent_idx")

	b.Validate(&Events.Service, validate.Required[string]())
	b.Validate(&Events.Action, validate.Required[string]())
	b.Validate(&Events.ActorID, validate.Required[string]())
	b.Validate(&Events.Outcome, validate.OneOf(OutcomeSuccess, OutcomeDenied, OutcomeFailure))

	// A foreign key to a partitioned table needs the whole key, which a relation
	// cannot express, so the relations below only say how to load rows, and the
	// constraint is declared on Person.
	b.HasMany(&Events.Persons).
		ForeignKey(&Persons.EventID).
		References(&Events.ID).
		NoForeignKey().
		NoIndex()

	b.HasMany(&Events.Children).
		ForeignKey(&Events.ParentID).
		References(&Events.ID).
		NoForeignKey().
		NoIndex()
}
