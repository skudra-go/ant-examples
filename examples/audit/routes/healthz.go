package routes

import (
	"net/http"

	audit "github.com/skudra-go/ant-examples/audit"
)

func health(app *audit.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := app.DB.DB().PingContext(r.Context()); err != nil {
			writeJSON(
				w,
				http.StatusServiceUnavailable,
				map[string]string{"status": "database unreachable"},
			)

			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
