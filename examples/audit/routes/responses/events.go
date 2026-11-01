// Package responses holds the shapes the API returns. Each embeds its model and
// adds what a client needs.
package responses

import (
	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/audit/models"
)

// Event is an event with its persons back inside it, as the sender gave it.
type Event struct {
	models.Event
	Persons []models.Person `json:"persons"`
}

// Events is a page of events.
type Events struct {
	Items []Event `json:"items"`
	Next  string  `json:"next,omitempty"`
}

// NewEvent shapes an event.
func NewEvent(e *models.Event) Event {
	persons := e.Persons
	if persons == nil {
		persons = []models.Person{}
	}

	return Event{Event: *e, Persons: persons}
}

// NewEvents shapes a page.
func NewEvents(page *ant.CursorPage[models.Event]) Events {
	out := Events{Items: make([]Event, len(page.Items)), Next: page.Next}

	for i, e := range page.Items {
		out.Items[i] = NewEvent(e)
	}

	return out
}
