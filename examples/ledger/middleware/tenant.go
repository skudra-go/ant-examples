package middleware

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Tenant checks the token, and puts the caller's tenant in the request's context.
// From here on, every query sees only that tenant's rows, through the global filter
// on models.Tenanted. A real service would read the tenant from the token.
func Tenant(token string) func(http.Handler) http.Handler {
	want := []byte(token)

	refuse := func(w http.ResponseWriter, status int, message string) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"`+message+`"}`, status)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				refuse(w, http.StatusUnauthorized, "a valid token is required")

				return
			}

			tenant, err := strconv.ParseInt(r.Header.Get("X-Tenant"), 10, 64)
			if err != nil {
				refuse(w, http.StatusBadRequest, "X-Tenant must be a tenant id")

				return
			}

			next.ServeHTTP(w, r.WithContext(models.WithTenant(r.Context(), tenant)))
		})
	}
}
