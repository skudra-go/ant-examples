package routes

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/skudra-go/ant-examples/blog/services"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// fail turns a service error into a status. Anything unexpected is a 500 with no
// detail, because database errors can name tables and values.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, services.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, services.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

// read decodes a JSON body, and answers the request itself if it cannot.
func read[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var body T
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "the body is not valid JSON"},
		)

		return body, false
	}

	return body, true
}
