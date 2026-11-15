// Package api is a slice of listmonk's admin API, on the ported core. Each
// request gets its own ant session, so tracked entities never leak between
// requests and two requests never share a tracker.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

// Server serves the API.
type Server struct {
	db  *models.AppDB
	log *slog.Logger
}

// New returns the API on db, the root opened with ant.Open.
func New(db *models.AppDB, log *slog.Logger) http.Handler {
	s := &Server{db: db, log: log}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/subscribers", s.querySubscribers)
	mux.HandleFunc("GET /api/subscribers/{id}", s.getSubscriber)
	mux.HandleFunc("POST /api/subscribers", s.createSubscriber)
	mux.HandleFunc("PUT /api/subscribers/{id}", s.updateSubscriber)
	mux.HandleFunc("PUT /api/subscribers/query/blocklist", s.blocklistByQuery)
	mux.HandleFunc("GET /api/lists", s.queryLists)
	mux.HandleFunc("POST /api/lists", s.createList)
	mux.HandleFunc("PUT /api/lists/{id}", s.updateList)
	mux.HandleFunc("PUT /api/campaigns/{id}/status", s.campaignStatus)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("GET /link/{link}/{campaign}/{subscriber}", s.linkRedirect)

	return mux
}

// core returns a core on a new session: one unit of work per request. The
// root has tracking off, and a session inherits that, so it is turned on
// here: without it, SaveChanges would find nothing to save and return nil.
func (s *Server) core() *core.Core {
	return core.New(ant.Session(s.db, ant.WithTracking(true)))
}

// envelope is listmonk's response shape: {"data": ...}.
type envelope struct {
	Data any `json:"data"`
}

func (s *Server) ok(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, core.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, core.ErrExists):
		status = http.StatusConflict
	case errors.Is(err, core.ErrInvalid):
		status = http.StatusBadRequest
	}

	if status == http.StatusInternalServerError {
		s.log.ErrorContext(
			r.Context(),
			"request failed",
			"path",
			r.URL.Path,
			"error",
			err,
			"sql",
			ant.FailedSQL(err),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
}

func pathID(r *http.Request) (int32, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id < 1 {
		return 0, core.ErrInvalid
	}

	return int32(id), nil
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errors.Join(core.ErrInvalid, err)
	}

	return nil
}

// ids reads repeated or comma-separated integer query parameters.
func ids(values []string) ([]int32, error) {
	var out []int32

	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if part == "" {
				continue
			}

			id, err := strconv.ParseInt(part, 10, 32)
			if err != nil {
				return nil, core.ErrInvalid
			}

			out = append(out, int32(id))
		}
	}

	return out, nil
}

// paging reads page and per_page; per_page=all means no limit.
func paging(r *http.Request) (offset, limit, page int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	perPage := r.URL.Query().Get("per_page")
	if perPage == "all" {
		return 0, 0, 1
	}

	limit, _ = strconv.Atoi(perPage)
	if limit < 1 || limit > 1000 {
		limit = 20
	}

	return (page - 1) * limit, limit, page
}
