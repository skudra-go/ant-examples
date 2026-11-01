package routes

import (
	"net/http"
	"time"

	audit "github.com/skudra-go/ant-examples/audit"
)

type adminRoutes struct {
	app *audit.App
}

func (a adminRoutes) partitions(w http.ResponseWriter, r *http.Request) {
	months, err := a.app.Retain.Months(r.Context())
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{"cutoff": a.app.Retain.Cutoff(time.Now()), "partitions": months},
	)
}

func (a adminRoutes) cleanup(w http.ResponseWriter, r *http.Request) {
	report, err := a.app.Retain.Run(r.Context(), time.Now())
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, report)
}
