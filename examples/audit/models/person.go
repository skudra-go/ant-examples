package models

import (
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/ids"
	"github.com/skudra-go/ant/validate"
)

// Persons is the field probe for Person.
var Persons = ant.Fields[Person]()

// Role values: what happened to a person in an event.
const (
	RoleAffected = "affected"
	RoleViewed   = "viewed"
	RoleExported = "exported"
	RoleNotified = "notified"
)

// Person is someone an event touched. The API receives persons inside the event,
// and they are stored apart from it, so that "everything that happened to this
// person" is one indexed query. OccurredAt is copied from the event, to split this
// table by month in step with its parent.
type Person struct {
	ID         string                 `json:"id"`
	EventID    string                 `json:"eventId"`
	OccurredAt time.Time              `json:"occurredAt"`
	Ref        string                 `json:"ref"`
	Name       *string                `json:"name,omitempty"`
	Role       string                 `json:"role"`
	Fields     postgres.Array[string] `json:"fields"`

	Event *Event `json:"-"`
}

// Configure says what the struct cannot.
func (Person) Configure(b *ant.ModelBuilder[Person]) {
	b.Table("event_persons")
	ant.GenerateKey(b, &Persons.ID, ids.UUID)
	b.CompositeKey(ant.F(&Persons.ID), ant.F(&Persons.OccurredAt))
	b.PartitionByRange(ant.F(&Persons.OccurredAt))

	b.Column(&Persons.ID).Type("uuid")
	b.Column(&Persons.EventID).Type("uuid")

	b.Column(&Persons.Role).
		Enum("person_role", RoleAffected, RoleViewed, RoleExported, RoleNotified)

	b.Column(&Persons.Fields).Sensitive()

	b.Index(&Persons.Ref).And(&Persons.OccurredAt).Desc()
	b.Validate(&Persons.Ref, validate.Required[string]())
	b.Validate(&Persons.Role, validate.OneOf(RoleAffected, RoleViewed, RoleExported, RoleNotified))

	b.BelongsTo(&Persons.Event).
		ForeignKey(&Persons.EventID).
		References(&Events.ID).
		NoForeignKey().
		NoIndex()

	b.ForeignKey(ant.F(&Persons.EventID), ant.F(&Persons.OccurredAt)).
		References(ant.F(&Events.ID), ant.F(&Events.OccurredAt)).
		OnDelete(ant.Cascade)
}
