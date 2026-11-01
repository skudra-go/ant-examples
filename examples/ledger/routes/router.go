// Package routes is the HTTP layer.
package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	ledger "github.com/skudra-go/ant-examples/ledger"
	"github.com/skudra-go/ant-examples/ledger/middleware"
	"github.com/skudra-go/ant-examples/ledger/models"
	"github.com/skudra-go/ant-examples/ledger/routes/requests"
	"github.com/skudra-go/ant-examples/ledger/routes/responses"
	"github.com/skudra-go/ant-examples/ledger/services"
)

// New returns the API.
func New(app *ledger.App) http.Handler {
	api := http.NewServeMux()
	h := handlers{app}
	api.HandleFunc("GET /accounts", h.chart)
	api.HandleFunc("POST /accounts", h.createAccount)
	api.HandleFunc("GET /accounts/{code}", h.subtree)
	api.HandleFunc("PATCH /accounts/{code}", h.rename)
	api.HandleFunc("DELETE /accounts/{code}", h.closeAccount)
	api.HandleFunc("POST /accounts/{code}/reopen", h.reopen)
	api.HandleFunc("GET /accounts/{code}/statement", h.statement)
	api.HandleFunc("POST /accounts/{code}/statement", h.importStatement)
	api.HandleFunc("PUT /chart", h.importChart)
	api.HandleFunc("POST /transfers", h.transfer)
	api.HandleFunc("POST /transfers/preview", h.preview)
	api.HandleFunc("POST /entries", h.entry)
	api.HandleFunc("POST /entries/{number}/reverse", h.reverse)
	api.HandleFunc("GET /reports/trial-balance", h.trial)
	api.HandleFunc("GET /reports/recent", h.recent)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.Handle("/", middleware.Tenant(app.Config.APIToken)(api))
	handler := http.Handler(mux)
	handler = middleware.Logging(app.Log)(handler)

	return middleware.Recover(app.Log)(handler)
}

type handlers struct {
	app *ledger.App
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func fail(w http.ResponseWriter, err error) {
	message := map[string]string{"error": err.Error()}

	switch {
	case errors.Is(err, services.ErrNotFound):
		writeJSON(w, http.StatusNotFound, message)
	case errors.Is(err, services.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, message)
	case errors.Is(err, services.ErrInsufficientFunds):
		writeJSON(w, http.StatusUnprocessableEntity, message)
	case errors.Is(err, services.ErrConflict):
		writeJSON(w, http.StatusConflict, message)
	case errors.Is(err, models.ErrNoTenant):
		writeJSON(w, http.StatusBadRequest, message)
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func read[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var body T
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad body: " + err.Error()})

		return body, false
	}

	return body, true
}

func respond[T any](w http.ResponseWriter, status int, value T, err error) {
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, status, value)
}

func (h handlers) health(w http.ResponseWriter, r *http.Request) {
	if err := h.app.DB.DB().PingContext(r.Context()); err != nil {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			map[string]string{"status": "database unreachable"},
		)

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h handlers) chart(w http.ResponseWriter, r *http.Request) {
	tree, err := h.app.Accounts.Tree(r.Context())
	respond(w, http.StatusOK, tree, err)
}

func (h handlers) createAccount(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Account](w, r)
	if !ok {
		return
	}

	account, err := h.app.Accounts.Create(r.Context(), services.AccountInput{
		Code:     body.Code,
		Name:     body.Name,
		Kind:     body.Kind,
		Currency: body.Currency,
		Parent:   body.Parent,
		IBAN:     body.IBAN,
		Floor:    body.Floor,
	})
	respond(w, http.StatusCreated, account, err)
}

func (h handlers) subtree(w http.ResponseWriter, r *http.Request) {
	accounts, total, err := h.app.Accounts.Subtree(r.Context(), r.PathValue("code"))
	respond(w, http.StatusOK, responses.Subtree{Accounts: accounts, Total: total}, err)
}

func (h handlers) rename(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Rename](w, r)
	if !ok {
		return
	}

	account, err := h.app.Accounts.Rename(r.Context(), r.PathValue("code"), body.Name, body.Version)
	respond(w, http.StatusOK, account, err)
}

func (h handlers) closeAccount(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Accounts.Close(r.Context(), r.PathValue("code")); err != nil {
		fail(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handlers) reopen(w http.ResponseWriter, r *http.Request) {
	if err := h.app.Accounts.Reopen(r.Context(), r.PathValue("code")); err != nil {
		fail(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handlers) statement(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	lines, err := h.app.Reports.Statement(
		r.Context(),
		r.PathValue("code"),
		max(page, 1),
		min(max(size, 20), 200),
	)
	if err != nil {
		fail(w, err)

		return
	}

	writeJSON(w, http.StatusOK, responses.NewStatement(lines))
}

func (h handlers) importStatement(w http.ResponseWriter, r *http.Request) {
	body, ok := read[[]requests.BankLine](w, r)
	if !ok {
		return
	}

	lines := make([]services.BankLine, len(body))

	for i, line := range body {
		lines[i] = services.BankLine{
			Reference: line.Reference,
			Amount:    line.Amount,
			BookedOn:  line.BookedOn,
		}
	}

	result, err := h.app.Reconcile.Import(r.Context(), r.PathValue("code"), lines)
	respond(w, http.StatusOK, result, err)
}

func toNodes(nodes []requests.ChartNode) []services.ImportNode {
	out := make([]services.ImportNode, len(nodes))

	for i, node := range nodes {
		out[i] = services.ImportNode{
			Code:     node.Code,
			Name:     node.Name,
			Kind:     node.Kind,
			Currency: node.Currency,
			Children: toNodes(node.Children),
		}
	}

	return out
}

func (h handlers) importChart(w http.ResponseWriter, r *http.Request) {
	body, ok := read[[]requests.ChartNode](w, r)
	if !ok {
		return
	}

	created, updated, err := h.app.Accounts.Import(r.Context(), toNodes(body))
	respond(w, http.StatusOK, map[string]int{"created": created, "updated": updated}, err)
}

func (h handlers) transfer(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Transfer](w, r)
	if !ok {
		return
	}

	entry, err := h.app.Journal.Post(
		r.Context(),
		services.Transfer(body.From, body.To, body.Amount, body.Memo, body.Key),
	)
	respond(w, http.StatusCreated, entry, err)
}

func (h handlers) preview(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Transfer](w, r)
	if !ok {
		return
	}

	preview, err := h.app.Journal.Preview(
		r.Context(),
		services.Transfer(body.From, body.To, body.Amount, body.Memo, body.Key),
	)
	respond(w, http.StatusOK, preview, err)
}

func (h handlers) entry(w http.ResponseWriter, r *http.Request) {
	body, ok := read[requests.Entry](w, r)
	if !ok {
		return
	}

	input := services.EntryInput{Memo: body.Memo, Key: body.Key}

	for _, line := range body.Lines {
		input.Lines = append(
			input.Lines,
			services.LineInput{Account: line.Account, Amount: line.Amount},
		)
	}

	entry, err := h.app.Journal.Post(r.Context(), input)
	respond(w, http.StatusCreated, entry, err)
}

func (h handlers) reverse(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.ParseInt(r.PathValue("number"), 10, 64)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{"error": "the number must be a number"},
		)

		return
	}

	body, ok := read[requests.Reverse](w, r)
	if !ok {
		return
	}

	entry, err := h.app.Journal.Reverse(r.Context(), number, body.Memo)
	respond(w, http.StatusCreated, entry, err)
}

func (h handlers) trial(w http.ResponseWriter, r *http.Request) {
	report, err := h.app.Reports.Trial(r.Context(), time.Now())
	respond(w, http.StatusOK, report, err)
}

func (h handlers) recent(w http.ResponseWriter, r *http.Request) {
	activity, err := h.app.Reports.Recent(r.Context())
	respond(w, http.StatusOK, activity, err)
}
