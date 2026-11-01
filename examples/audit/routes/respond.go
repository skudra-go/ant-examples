package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/skudra-go/ant-examples/audit/services"
)

const (
	defaultPageSize = 50
	largestPageSize = 500
	largestBatch    = 5000
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, services.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

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

func number(r *http.Request, name string, fallback int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil && n > 0 {
		return n
	}

	return fallback
}

// period reads from and to, defaulting to the last 30 days.
func period(r *http.Request) (time.Time, time.Time) {
	to, err := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err != nil {
		to = time.Now().UTC()
	}

	from, err := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	if err != nil {
		from = to.AddDate(0, 0, -30)
	}

	return from, to
}

// filter reads the search parameters. Each meta.key=value pair matches inside the
// event's metadata.
func filter(r *http.Request) services.Filter {
	q := r.URL.Query()

	f := services.Filter{
		Service: q.Get("service"),
		Action:  q.Get("action"),
		Outcome: q.Get("outcome"),
		Actor:   q.Get("actor"),
		Person:  q.Get("person"),
		Role:    q.Get("role"),
		Text:    q.Get("q"),
	}

	if text := q.Get("from"); text != "" {
		f.From, _ = time.Parse(time.RFC3339, text)
	}

	if text := q.Get("to"); text != "" {
		f.To, _ = time.Parse(time.RFC3339, text)
	}

	for name, values := range q {
		if key, ok := strings.CutPrefix(name, "meta."); ok && len(values) > 0 {
			if f.Meta == nil {
				f.Meta = map[string]any{}
			}

			f.Meta[key] = values[0]
		}
	}

	return f
}
