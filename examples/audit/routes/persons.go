package routes

import (
	"fmt"
	"net/http"

	audit "github.com/skudra-go/ant-examples/audit"
	"github.com/skudra-go/ant-examples/audit/routes/requests"
	"github.com/skudra-go/ant-examples/audit/services"
)

type personRoutes struct {
	app *audit.App
}

func (p personRoutes) summary(w http.ResponseWriter, r *http.Request) {
	summary, err := p.app.Persons.Summary(r.Context(), r.PathValue("ref"))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (p personRoutes) timeline(w http.ResponseWriter, r *http.Request) {
	steps, err := p.app.Persons.Timeline(
		r.Context(),
		r.PathValue("ref"),
		min(number(r, "size", defaultPageSize), largestPageSize),
	)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, steps)
}

func (p personRoutes) erase(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Erase](w, r)
	if !ok {
		return
	}

	if body.RequestedBy == "" {
		fail(w, fmt.Errorf("%w: say who requested this", services.ErrInvalid))

		return
	}

	erasure, err := p.app.Persons.Erase(r.Context(), r.PathValue("ref"), body.RequestedBy)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, erasure)
}
