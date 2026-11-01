package middleware

import (
	"net/http"

	"github.com/skudra-go/ant"
)

// Watch gives each request its own count of statements, so ant.WithDiagnostics can
// warn about an N+1 pattern or a request that spent too long in the database.
func Watch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(ant.Watch(r.Context())))
	})
}
