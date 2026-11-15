package core_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

func TestQueryListsMatchesUpstream(t *testing.T) {
	c, db := setup(t)
	audienceFixture(t, c)
	ctx := t.Context()

	if err := c.RefreshListStats(ctx); err != nil {
		t.Fatal(err)
	}

	type upstreamList struct {
		ID                 int32
		Name               string
		SubscriberCount    int64
		SubscriberStatuses string
	}

	text := strings.ReplaceAll(upstream(t, "query-lists"), "%order%", "subscriber_count DESC, id")

	want, err := ant.Raw[upstreamList](
		db,
		text,
		0,
		"",
		"",
		"",
		"",
		[]string{},
		true,
		[]int32{},
		0,
		0,
	).All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	got, total, err := c.QueryLists(ctx, core.ListQuery{}, "subscriber_count", "desc", 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if int(total) != len(want) || len(got) != len(want) {
		t.Fatalf("got %d (total %d), upstream %d", len(got), total, len(want))
	}

	for i, w := range want {
		g := got[i]

		var statuses map[models.SubscriptionStatus]int64
		if err := json.Unmarshal([]byte(w.SubscriberStatuses), &statuses); err != nil {
			t.Fatal(err)
		}

		if g.ID != w.ID || g.SubscriberCount != w.SubscriberCount ||
			!maps.Equal(g.SubscriberStatuses, statuses) {
			t.Errorf(
				"row %d: got %d %d %v, upstream %+v",
				i,
				g.ID,
				g.SubscriberCount,
				g.SubscriberStatuses,
				w,
			)
		}
	}

	// A tag filter, and a type filter.
	tagged, _, err := c.QueryLists(
		ctx,
		core.ListQuery{Tags: []string{"tag1"}, Type: models.ListPublic},
		"name",
		"asc",
		0,
		0,
	)
	if err != nil || len(tagged) != 1 || tagged[0].Name != "Product news" {
		t.Errorf("tagged: %+v %v", tagged, err)
	}
}

func TestSettings(t *testing.T) {
	c, db := setup(t)
	ctx := t.Context()

	got, err := c.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := ant.Raw[string](db, upstream(t, "get-settings")).First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var want map[string]any
	if err := json.Unmarshal([]byte(*raw), &want); err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) || len(got) < 50 {
		t.Fatalf("got %d keys, upstream %d", len(got), len(want))
	}

	for k, v := range want {
		var g any
		if err := json.Unmarshal(got[k], &g); err != nil || !reflect.DeepEqual(g, v) {
			t.Errorf("%s: got %s, upstream %v", k, got[k], v)
		}
	}

	n, err := c.UpdateSettings(ctx, map[string]json.RawMessage{
		"app.site_name":            json.RawMessage(`"Ported"`),
		"app.concurrency":          json.RawMessage(`20`),
		"no.such.key":              json.RawMessage(`true`),
		"privacy.domain_blocklist": json.RawMessage(`["spam.example"]`),
	})
	if err != nil || n != 3 {
		t.Fatalf("updated %d, %v", n, err)
	}

	got, err = c.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if string(got["app.site_name"]) != `"Ported"` || string(got["app.concurrency"]) != `20` {
		t.Errorf("not updated: %s %s", got["app.site_name"], got["app.concurrency"])
	}

	if _, ok := got["no.such.key"]; ok {
		t.Error("unknown key inserted")
	}
}

func TestLinks(t *testing.T) {
	c, db := setup(t)
	_, camp := audienceFixture(t, c)
	ctx := t.Context()

	first, err := c.CreateLink(ctx, "https://listmonk.app/docs")
	if err != nil {
		t.Fatal(err)
	}

	again, err := c.CreateLink(ctx, "https://listmonk.app/docs")
	if err != nil || again != first {
		t.Fatalf("second create: %q %v, want %q", again, err, first)
	}

	sub, err := c.GetSubscriber(ctx, 1, "", "")
	if err != nil {
		t.Fatal(err)
	}

	url, err := c.RegisterLinkClick(ctx, first, camp.UUID, sub.UUID)
	if err != nil || url != "https://listmonk.app/docs" {
		t.Fatalf("click: %q %v", url, err)
	}

	// Upstream's statement, for the same click with an unknown subscriber.
	upURL, err := ant.Raw[string](
		db,
		upstream(t, "register-link-click"),
		first,
		camp.UUID,
		"",
	).First(ctx)
	if err != nil || *upURL != url {
		t.Fatalf("upstream: %v %v", upURL, err)
	}

	mine, err := c.RegisterLinkClick(ctx, first, camp.UUID, "")
	if err != nil || mine != url {
		t.Fatal(err)
	}

	type click struct {
		CampaignID   *int32
		SubscriberID *int32
	}

	rows, err := ant.Raw[click](
		db,
		`SELECT campaign_id, subscriber_id FROM link_clicks ORDER BY id DESC LIMIT 2`,
	).All(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(rows[0], rows[1]) || rows[0].SubscriberID != nil ||
		*rows[0].CampaignID != camp.ID {
		t.Errorf("ported click %+v, upstream click %+v", rows[0], rows[1])
	}
}

type bounceState struct {
	Bounces        int64
	Status         string
	Unsubscribed   int64
	SubscriberGone bool
}

func readBounceState(t *testing.T, db *models.AppDB, subID int32) bounceState {
	t.Helper()

	r, err := ant.Raw[bounceState](
		db,
		`SELECT
		(SELECT count(*) FROM bounces WHERE subscriber_id = @id) AS bounces,
		COALESCE((SELECT status::text FROM subscribers WHERE id = @id), '') AS status,
		(SELECT count(*) FROM subscriber_lists WHERE subscriber_id = @id AND status = 'unsubscribed') AS unsubscribed,
		NOT EXISTS (SELECT 1 FROM subscribers WHERE id = @id) AS subscriber_gone`,
		sqlNamed("id", subID),
	).First(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return *r
}

func TestRecordBounceMatchesUpstream(t *testing.T) {
	for _, action := range []core.BounceAction{core.BounceBlocklist, core.BounceUnsubscribe, core.BounceDelete, core.BounceNone} {
		t.Run(string(action), func(t *testing.T) {
			c, db := setup(t)
			s, camp := audienceFixture(t, c)
			ctx := t.Context()
			sub := s.Subscribers[1]
			at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

			var want []bounceState

			err := db.Tx(ctx, func(tx *models.AppDB) error {
				for range 4 {
					// Each in a savepoint: once the subscriber is deleted,
					// upstream's statement fails on a NULL subscriber_id,
					// where the port reports not found.
					err := tx.Tx(ctx, func(tx *models.AppDB) error {
						_, err := ant.Raw[any](
							tx,
							upstream(t, "record-bounce"),
							sub.UUID,
							"",
							camp.UUID,
							"soft",
							"api",
							`{}`,
							at,
							2,
							string(action),
						).Exec(ctx)

						return err
					})
					if err != nil && !strings.Contains(err.Error(), "23502") {
						return err
					}

					want = append(want, readBounceState(t, tx, sub.ID))
				}

				return errRollback
			})
			if !errors.Is(err, errRollback) {
				t.Fatal(err)
			}

			var got []bounceState

			for range 4 {
				err := c.RecordBounce(
					ctx,
					core.BounceInput{
						SubscriberUUID: sub.UUID,
						CampaignUUID:   camp.UUID,
						Type:           models.BounceSoft,
						Source:         "api",
						CreatedAt:      at,
					},
					2,
					action,
				)
				if err != nil && !errors.Is(err, core.ErrNotFound) {
					t.Fatal(err)
				}

				got = append(got, readBounceState(t, db, sub.ID))
			}

			if !slices.Equal(got, want) {
				t.Errorf("got      %+v\nupstream %+v", got, want)
			}
		})
	}
}

func TestQueryBounces(t *testing.T) {
	c, _ := setup(t)
	_, camp := audienceFixture(t, c)
	ctx := t.Context()

	if err := c.RecordBounce(
		ctx,
		core.BounceInput{Email: "john@example.com", Type: models.BounceHard},
		5,
		core.BounceNone,
	); err != nil {
		t.Fatal(err)
	}

	if err := c.RecordBounce(
		ctx,
		core.BounceInput{
			Email:        "anon@example.com",
			CampaignUUID: camp.UUID,
			Type:         models.BounceSoft,
		},
		5,
		core.BounceNone,
	); err != nil {
		t.Fatal(err)
	}

	rows, err := c.QueryBounces(ctx, 0, 0, 10)
	if err != nil || len(rows) != 2 {
		t.Fatalf("%d rows, %v", len(rows), err)
	}

	names := map[string]string{}

	for _, r := range rows {
		names[r.Email] = fmt.Sprint(r.CampaignName)

		if r.CampaignName != nil {
			names[r.Email] = *r.CampaignName
		}
	}

	if names["john@example.com"] != "<nil>" || names["anon@example.com"] != "Fixture" {
		t.Errorf("campaign names: %v", names)
	}
}

func TestCampaignViewsByDay(t *testing.T) {
	c, db := setup(t)
	_, camp := audienceFixture(t, c)
	ctx := t.Context()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	for i, offset := range []time.Duration{1 * time.Hour, 5 * time.Hour, 49 * time.Hour, 50 * time.Hour, 51 * time.Hour} {
		view := &models.CampaignView{CampaignID: camp.ID, CreatedAt: day.Add(offset)}

		if i == 0 {
			view.SubscriberID = new(int32(1))
		}

		if err := db.CampaignViews.InsertOnly(ctx, view); err != nil {
			t.Fatal(err)
		}
	}

	full, err := c.CampaignViewsByDay(ctx, camp.ID, day, day.Add(3*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	counts := make([]int64, len(full))

	for i, d := range full {
		counts[i] = d.Count
	}

	if !slices.Equal(counts, []int64{2, 0, 3, 0}) {
		t.Errorf("with empty days: %v", full)
	}

	typed, err := c.CampaignViewsByDayTyped(ctx, camp.ID, day, day.Add(3*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if len(typed) != 2 || typed[0].Count != 2 || typed[1].Count != 3 ||
		!typed[1].Day.Equal(day.Add(48*time.Hour)) {
		t.Errorf("typed: %+v", typed)
	}
}
