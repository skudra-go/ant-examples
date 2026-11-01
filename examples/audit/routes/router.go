// Package routes is the HTTP layer.
package routes

import (
	"net/http"

	audit "github.com/skudra-go/ant-examples/audit"
	"github.com/skudra-go/ant-examples/audit/middleware"
)

// New returns the API.
func New(app *audit.App) http.Handler {
	mux := http.NewServeMux()
	ingest := middleware.RequireToken(app.Config.IngestToken)
	admin := middleware.RequireToken(app.Config.AdminToken)

	mux.HandleFunc("GET /healthz", health(app))
	mux.Handle("GET /metrics", admin(app.Metrics))

	events := eventRoutes{app}
	mux.Handle("POST /events", ingest(http.HandlerFunc(events.create)))
	mux.Handle("POST /events/batch", ingest(http.HandlerFunc(events.batch)))
	mux.Handle("GET /events", admin(http.HandlerFunc(events.search)))
	mux.Handle("GET /events/{id}", admin(http.HandlerFunc(events.get)))
	mux.Handle("GET /events/{id}/chain", admin(http.HandlerFunc(events.chain)))
	mux.Handle("GET /export", admin(http.HandlerFunc(events.export)))

	persons := personRoutes{app}
	mux.Handle("GET /persons/{ref}", admin(http.HandlerFunc(persons.summary)))
	mux.Handle("GET /persons/{ref}/timeline", admin(http.HandlerFunc(persons.timeline)))
	mux.Handle("POST /persons/{ref}/erase", admin(http.HandlerFunc(persons.erase)))

	stats := statRoutes{app}
	mux.Handle("GET /stats/activity", admin(http.HandlerFunc(stats.activity)))
	mux.Handle("GET /stats/rollup", admin(http.HandlerFunc(stats.rollup)))
	mux.Handle("GET /stats/top-actors", admin(http.HandlerFunc(stats.topActors)))
	mux.Handle("GET /stats/anomalies", admin(http.HandlerFunc(stats.anomalies)))
	mux.Handle("GET /stats/latest", admin(http.HandlerFunc(stats.latest)))

	retention := adminRoutes{app}
	mux.Handle("GET /admin/partitions", admin(http.HandlerFunc(retention.partitions)))
	mux.Handle("POST /admin/cleanup", admin(http.HandlerFunc(retention.cleanup)))

	handler := http.Handler(mux)
	handler = middleware.Watch(handler)
	handler = middleware.Logging(app.Log)(handler)

	return middleware.Recover(app.Log)(handler)
}
