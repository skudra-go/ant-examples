package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireToken lets a request through if it carries the admin token as a bearer
// token. A real blog would have accounts, but this keeps the example about ant.
func RequireToken(token string) func(http.Handler) http.Handler {
	want := []byte(token)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"a valid token is required"}`, http.StatusUnauthorized)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
