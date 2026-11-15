package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/anttest"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/listmonk/api"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

// root is opened as the server opens it: tracking off.
var root *models.AppDB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println(
			"TEST_DATABASE_URL is not set: the API tests need a database made by listmonk --install",
		)

		os.Exit(0)
	}

	db, err := ant.Open[models.AppDB](
		context.Background(),
		postgres.Dialect,
		dsn,
		ant.WithTracking(false),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	root = db
	code := m.Run()
	_ = db.Close()

	os.Exit(code)
}

// server returns the API on a transaction that rolls back after the test.
// The per-request sessions are taken from it, so they share it.
func server(t *testing.T) http.Handler {
	t.Helper()

	return api.New(anttest.Isolate(t, root), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func call(t *testing.T, h http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var buf bytes.Buffer

	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, &buf))

	var out map[string]any

	_ = json.Unmarshal(rec.Body.Bytes(), &out)

	return rec.Code, out
}

func data(t *testing.T, out map[string]any) map[string]any {
	t.Helper()

	d, ok := out["data"].(map[string]any)
	if !ok {
		t.Fatalf("no data in %v", out)
	}

	return d
}

func TestCreateAndUpdateSubscriber(t *testing.T) {
	h := server(t)

	code, out := call(t, h, "POST", "/api/subscribers", map[string]any{
		"email":                    "ada@example.org",
		"name":                     "Ada",
		"lists":                    []int{1, 2},
		"attribs":                  map[string]any{"city": "London"},
		"preconfirm_subscriptions": true,
	})

	if code != http.StatusOK {
		t.Fatalf("create: %d %v", code, out)
	}

	sub := data(t, out)
	if lists := sub["lists"].([]any); len(lists) != 2 {
		t.Fatalf("lists: %v", lists)
	}

	created := sub["created_at"].(string)
	if !strings.HasSuffix(created, "Z") {
		t.Errorf("created_at not in UTC: %s", created)
	}

	id := int(sub["id"].(float64))

	// The same e-mail again: 409.
	code, _ = call(
		t,
		h,
		"POST",
		"/api/subscribers",
		map[string]any{"email": "ada@example.org", "name": "Ada"},
	)

	if code != http.StatusConflict {
		t.Errorf("duplicate: %d", code)
	}

	// Rename, and keep only list 2. This goes through SaveChanges on the
	// request's session.
	code, out = call(t, h, "PUT", fmt.Sprintf("/api/subscribers/%d", id), map[string]any{
		"name":  "Ada Lovelace",
		"lists": []int{2},
	})

	if code != http.StatusOK {
		t.Fatalf("update: %d %v", code, out)
	}

	// A new request, a new session: what the database holds.
	code, out = call(t, h, "GET", fmt.Sprintf("/api/subscribers/%d", id), nil)
	if code != http.StatusOK {
		t.Fatalf("get: %d", code)
	}

	got := data(t, out)
	lists := got["lists"].([]any)

	if got["name"] != "Ada Lovelace" || len(lists) != 1 ||
		lists[0].(map[string]any)["id"].(float64) != 2 {
		t.Errorf("after update: %v", got)
	}

	if lists[0].(map[string]any)["subscription_status"] != "confirmed" {
		t.Errorf("a confirmed subscription lost its status: %v", lists[0])
	}
}

func TestQuerySubscribers(t *testing.T) {
	h := server(t)

	for _, email := range []string{"b@example.org", "c@example.org"} {
		if code, out := call(t, h, "POST", "/api/subscribers", map[string]any{
			"email":   email,
			"name":    email,
			"lists":   []int{1},
			"attribs": map[string]any{"plan": "pro"},
		}); code != http.StatusOK {
			t.Fatalf("create: %d %v", code, out)
		}
	}

	code, out := call(
		t,
		h,
		"GET",
		"/api/subscribers?list_id=1&order_by=email&order=desc&per_page=1&page=2&query="+
			"subscribers.attribs->>'plan'%3D'pro'",
		nil,
	)

	if code != http.StatusOK {
		t.Fatalf("query: %d %v", code, out)
	}

	page := data(t, out)
	results := page["results"].([]any)

	if page["total"].(float64) != 2 || len(results) != 1 ||
		results[0].(map[string]any)["email"] != "b@example.org" {
		t.Errorf("page: %v", page)
	}

	// A broken expression is the client's fault.
	code, _ = call(t, h, "GET", "/api/subscribers?query=no_such_column%3D1", nil)
	if code != http.StatusBadRequest {
		t.Errorf("bad query: %d", code)
	}
}

func TestLists(t *testing.T) {
	h := server(t)

	code, out := call(t, h, "POST", "/api/lists", map[string]any{
		"name":  "Beta",
		"type":  "public",
		"optin": "double",
		"tags":  []string{"beta"},
	})

	if code != http.StatusOK {
		t.Fatalf("create: %d %v", code, out)
	}

	id := int(data(t, out)["id"].(float64))

	code, out = call(
		t,
		h,
		"PUT",
		fmt.Sprintf("/api/lists/%d", id),
		map[string]any{"name": "Beta testers", "tags": []string{"beta", "early"}},
	)

	if code != http.StatusOK || data(t, out)["name"] != "Beta testers" {
		t.Fatalf("update: %d %v", code, out)
	}

	code, out = call(t, h, "GET", "/api/lists?tag=early&order_by=name", nil)
	if code != http.StatusOK {
		t.Fatalf("query: %d %v", code, out)
	}

	results := data(t, out)["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["name"] != "Beta testers" {
		t.Errorf("lists: %v", results)
	}
}

func TestStartCampaign(t *testing.T) {
	h := server(t)

	// The campaign listmonk's install made, with no send time: it runs.
	code, out := call(t, h, "PUT", "/api/campaigns/1/status", map[string]any{"status": "running"})
	if code != http.StatusOK || data(t, out)["status"] != "running" {
		t.Fatalf("start: %d %v", code, out)
	}

	code, _ = call(t, h, "PUT", "/api/campaigns/999999/status", map[string]any{"status": "running"})
	if code != http.StatusNotFound {
		t.Errorf("missing campaign: %d", code)
	}

	code, _ = call(t, h, "PUT", "/api/campaigns/1/status", map[string]any{"status": "finished"})
	if code != http.StatusBadRequest {
		t.Errorf("bad status: %d", code)
	}
}

func TestSettingsHidePasswords(t *testing.T) {
	h := server(t)

	code, out := call(t, h, "GET", "/api/settings", nil)
	if code != http.StatusOK {
		t.Fatalf("%d", code)
	}

	settings := data(t, out)
	if _, ok := settings["smtp"]; ok || settings["app.site_name"] == nil {
		t.Errorf("settings: smtp present=%v, site_name=%v", ok, settings["app.site_name"])
	}
}
