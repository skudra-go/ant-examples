package routes

import (
	"net/http"

	audit "github.com/skudra-go/ant-examples/audit"
)

type statRoutes struct {
	app *audit.App
}

func (s statRoutes) activity(w http.ResponseWriter, r *http.Request) {
	unit := r.URL.Query().Get("unit")
	if unit == "" {
		unit = "day"
	}

	from, to := period(r)

	buckets, err := s.app.Stats.Activity(r.Context(), unit, from, to)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, buckets)
}

func (s statRoutes) rollup(w http.ResponseWriter, r *http.Request) {
	from, to := period(r)

	totals, err := s.app.Stats.Rollup(r.Context(), from, to)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, totals)
}

func (s statRoutes) topActors(w http.ResponseWriter, r *http.Request) {
	from, to := period(r)

	actors, err := s.app.Stats.TopActors(r.Context(), from, to, number(r, "size", 10))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, actors)
}

func (s statRoutes) anomalies(w http.ResponseWriter, r *http.Request) {
	from, to := period(r)

	findings, err := s.app.Stats.Anomalies(r.Context(), from, to, int64(number(r, "persons", 20)))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, findings)
}

func (s statRoutes) latest(w http.ResponseWriter, r *http.Request) {
	latest, err := s.app.Stats.LatestByActor(r.Context(), number(r, "size", 10))
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, latest)
}
