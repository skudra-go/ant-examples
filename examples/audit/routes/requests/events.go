// Package requests holds the shapes the API accepts.
package requests

import "time"

// Person is someone an event touched. It arrives inside the event.
type Person struct {
	Ref    string   `json:"ref"`
	Name   *string  `json:"name"`
	Role   string   `json:"role"`
	Fields []string `json:"fields"`
}

// Event is the body of POST /events, and one element of POST /events/batch. A
// sender that sets ID can retry safely: the same ID is stored once.
type Event struct {
	ID         string         `json:"id"`
	OccurredAt time.Time      `json:"occurredAt"`
	Service    string         `json:"service"`
	Action     string         `json:"action"`
	Outcome    string         `json:"outcome"`
	Actor      Actor          `json:"actor"`
	Resource   Resource       `json:"resource"`
	IP         *string        `json:"ip"`
	ParentID   *string        `json:"parentId"`
	Summary    string         `json:"summary"`
	Metadata   map[string]any `json:"metadata"`
	Persons    []Person       `json:"persons"`
}

// Actor is who acted.
type Actor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Resource is what was acted on.
type Resource struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Erase is the body of POST /persons/{ref}/erase.
type Erase struct {
	RequestedBy string `json:"requestedBy"`
}
