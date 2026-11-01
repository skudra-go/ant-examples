package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	audit "github.com/skudra-go/ant-examples/audit"
	"github.com/skudra-go/ant-examples/audit/routes/requests"
	"github.com/skudra-go/ant-examples/audit/routes/responses"
	"github.com/skudra-go/ant-examples/audit/services"
)

type eventRoutes struct {
	app *audit.App
}

func toInput(e requests.Event) services.EventInput {
	input := services.EventInput{
		ID:           e.ID,
		OccurredAt:   e.OccurredAt,
		Service:      e.Service,
		Action:       e.Action,
		Outcome:      e.Outcome,
		ActorID:      e.Actor.ID,
		ActorName:    e.Actor.Name,
		ResourceType: e.Resource.Type,
		ResourceID:   e.Resource.ID,
		IP:           e.IP,
		ParentID:     e.ParentID,
		Summary:      e.Summary,
		Metadata:     e.Metadata,
	}

	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now()
	}

	for _, person := range e.Persons {
		input.Persons = append(input.Persons, services.PersonInput{
			Ref:    person.Ref,
			Name:   person.Name,
			Role:   person.Role,
			Fields: person.Fields,
		})
	}

	return input
}

func (e eventRoutes) create(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Event](w, r)
	if !ok {
		return
	}

	result, err := e.app.Ingest.Store(r.Context(), []services.EventInput{toInput(body)})
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

func (e eventRoutes) batch(w http.ResponseWriter, r *http.Request) {
	body, ok := read[[]requests.Event](w, r)
	if !ok {
		return
	}

	if len(body) > largestBatch {
		fail(w, fmt.Errorf("%w: at most %d events in a batch", services.ErrInvalid, largestBatch))

		return
	}

	inputs := make([]services.EventInput, len(body))

	for i, event := range body {
		inputs[i] = toInput(event)
	}

	result, err := e.app.Ingest.Store(r.Context(), inputs)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

func (e eventRoutes) search(w http.ResponseWriter, r *http.Request) {
	size := min(number(r, "size", defaultPageSize), largestPageSize)

	page, err := e.app.Search.Events(r.Context(), filter(r), r.URL.Query().Get("cursor"), size)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.NewEvents(page))
}

func (e eventRoutes) get(w http.ResponseWriter, r *http.Request) {
	event, err := e.app.Search.Event(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.NewEvent(event))
}

func (e eventRoutes) chain(w http.ResponseWriter, r *http.Request) {
	events, err := e.app.Search.Chain(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, events)
}

// export streams newline-delimited JSON, a row at a time, so its size does not
// matter to the server's memory.
func (e eventRoutes) export(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(w)

	for event, err := range e.app.Search.Export(r.Context(), filter(r)) {
		if err != nil {
			e.app.Log.Error("export failed", "error", err)

			return
		}

		if err := encoder.Encode(event); err != nil {
			return // the client went away
		}
	}
}
