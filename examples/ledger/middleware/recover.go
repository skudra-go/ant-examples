package middleware

import (
	"log/slog"
	"net/http"
)

// Recover turns a panic into a 500, so one bad request cannot stop the server.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if failure := recover(); failure != nil {
					log.Error("panic", "path", r.URL.Path, "panic", failure)
					http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
