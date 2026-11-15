package core_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

var errRollback = errors.New("rollback")

// idRow reads the id column of a row and ignores the others.
type idRow struct {
	ID int32
}

// audienceFixture adds subscribers with every subscription status to the
// seeded lists, and a running campaign over all lists.
func audienceFixture(t *testing.T, c *core.Core) (seeded, *models.Campaign) {
	t.Helper()

	ctx := t.Context()
	s := seed(t, c)

	statuses := []models.SubscriptionStatus{
		models.SubscriptionUnconfirmed,
		models.SubscriptionConfirmed,
		models.SubscriptionUnsubscribed,
	}

	for i := range 30 {
		sub := core.NewSubscriber(
			fmt.Sprintf("bulk%02d@example.org", i),
			fmt.Sprintf("Bulk %02d", i),
			nil,
		)

		if i%7 == 0 {
			sub.Status = models.SubscriberBlocklisted
		}

		lists := []int32{s.Lists[i%3].ID, 1 + int32(i%2)}
		if err := c.InsertSubscriber(ctx, sub, lists, statuses[i%3]); err != nil {
			t.Fatal(err)
		}
	}

	camp := &models.Campaign{
		Name:      "Fixture",
		Subject:   "Hello",
		FromEmail: "news@example.org",
		Body:      "<p>Hi</p>",
		Messenger: "email",
	}

	listIDs := []int32{1, 2, s.Lists[0].ID, s.Lists[1].ID, s.Lists[2].ID}
	if err := c.CreateCampaign(ctx, camp, listIDs); err != nil {
		t.Fatal(err)
	}

	if _, err := c.UpdateCampaignStatus(ctx, camp.ID, models.CampaignRunning); err != nil {
		t.Fatal(err)
	}

	return s, camp
}

func TestNextCampaignsMatchesUpstream(t *testing.T) {
	c, db := setup(t)
	_, camp := audienceFixture(t, c)
	ctx := t.Context()

	type counts struct {
		ToSend          int32
		MaxSubscriberID int32
		Status          string
	}

	read := func(h *models.AppDB) counts {
		r, err := ant.Raw[counts](
			h,
			`SELECT to_send, max_subscriber_id, status::text FROM campaigns WHERE id = ?`,
			camp.ID,
		).First(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return *r
	}

	var want counts

	err := db.Tx(ctx, func(tx *models.AppDB) error {
		rows, err := ant.Raw[idRow](
			tx,
			upstream(t, "next-campaigns"),
			[]int32{},
			[]int32{},
		).All(ctx)
		if err != nil {
			return err
		}

		if !slices.ContainsFunc(rows, func(r idRow) bool { return r.ID == camp.ID }) {
			t.Errorf("upstream did not pick the campaign: %v", rows)
		}

		want = read(tx)

		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}

	camps, err := c.NextCampaigns(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(camps) != 1 || camps[0].ID != camp.ID {
		t.Fatalf("picked %d campaigns", len(camps))
	}

	if got := read(db); got != want || got.ToSend == 0 {
		t.Errorf("got %+v, upstream %+v", got, want)
	}
}

func TestNextCampaignSubscribersMatchesUpstream(t *testing.T) {
	c, db := setup(t)
	s, camp := audienceFixture(t, c)
	ctx := t.Context()

	if _, err := c.NextCampaigns(ctx, nil, nil); err != nil {
		t.Fatal(err)
	}

	listIDs := []int32{1, 2, s.Lists[0].ID, s.Lists[1].ID, s.Lists[2].ID}

	var want []int32

	err := db.Tx(ctx, func(tx *models.AppDB) error {
		for {
			var cur struct {
				Last int32
				Max  int32
			}

			r, err := ant.Raw[struct {
				Last int32
				Max  int32
			}](tx, `SELECT last_subscriber_id AS last, max_subscriber_id AS max FROM campaigns WHERE id = ?`, camp.ID).First(ctx)
			if err != nil {
				return err
			}

			cur = *r

			batch, err := ant.Raw[idRow](
				tx,
				upstream(t, "next-campaign-subscribers"),
				camp.ID,
				"regular",
				cur.Last,
				cur.Max,
				listIDs,
				4,
			).All(ctx)
			if err != nil {
				return err
			}

			if len(batch) == 0 {
				return errRollback
			}

			for _, r := range batch {
				want = append(want, r.ID)
			}
		}
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}

	var got []int32

	for {
		batch, err := c.NextCampaignSubscribers(ctx, camp.ID, 4)
		if err != nil {
			t.Fatal(err)
		}

		if len(batch) == 0 {
			break
		}

		for _, sub := range batch {
			got = append(got, sub.ID)
		}
	}

	if len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("got %v\nupstream %v", got, want)
	}
}

func TestQueryCampaignsMatchesUpstream(t *testing.T) {
	c, db := setup(t)
	s, camp := audienceFixture(t, c)
	ctx := t.Context()

	type upstreamRow struct {
		ID    int32
		Total int64
		Lists string
	}

	run := func(statuses []string, tags []string, search string, listIDs []int32, all bool) []upstreamRow {
		text := strings.ReplaceAll(upstream(t, "query-campaigns"), "%order%", "c.created_at DESC")

		rows, err := ant.Raw[upstreamRow](
			db,
			"SELECT id, total, lists::text AS lists FROM ("+strings.TrimSuffix(
				strings.TrimSpace(text),
				";",
			)+") q",
			0,
			statuses,
			tags,
			search,
			all,
			listIDs,
			0,
			0,
		).All(ctx)
		if err != nil {
			t.Fatal(err)
		}

		return rows
	}

	cases := []struct {
		name string
		in   core.CampaignQuery
		want []upstreamRow
	}{
		{"all", core.CampaignQuery{}, run([]string{}, []string{}, "", []int32{}, true)},
		{
			"running",
			core.CampaignQuery{Statuses: []models.CampaignStatus{models.CampaignRunning}},
			run([]string{"running"}, []string{}, "", []int32{}, true),
		},
		{
			"tag",
			core.CampaignQuery{Tags: []string{"test-campaign"}},
			run([]string{}, []string{"test-campaign"}, "", []int32{}, true),
		},
		{
			"list",
			core.CampaignQuery{ListIDs: []int32{s.Lists[2].ID}},
			run([]string{}, []string{}, "", []int32{s.Lists[2].ID}, false),
		},
		{
			"search",
			core.CampaignQuery{Search: "Fixture"},
			run([]string{}, []string{}, "Fixture", []int32{}, true),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, total, err := c.QueryCampaigns(ctx, tc.in, "created_at", "desc", 0, 0)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("got %d campaigns, upstream %d", len(got), len(tc.want))
			}

			for i, row := range tc.want {
				if got[i].ID != row.ID || total != row.Total {
					t.Errorf("row %d: got id %d total %d, upstream %+v", i, got[i].ID, total, row)
				}

				if want := fmt.Sprint(
					len(got[i].Lists),
				); !strings.Contains(row.Lists, `"name"`) &&
					want != "0" {
					t.Errorf("lists differ: %s", row.Lists)
				}
			}
		})
	}

	_ = camp
}

func TestUpdateCampaignStatusSchedules(t *testing.T) {
	c, _ := setup(t)
	_, camp := audienceFixture(t, c)
	ctx := t.Context()

	later := time.Now().Add(time.Hour)

	if _, err := c.DB().
		Campaigns.Where(ant.Eq(&core.C.ID, camp.ID)).
		Update(ctx, ant.Set(&core.C.SendAt, &later)); err != nil {
		t.Fatal(err)
	}

	got, err := c.UpdateCampaignStatus(ctx, camp.ID, models.CampaignRunning)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != models.CampaignScheduled {
		t.Errorf("status %s, want scheduled", got.Status)
	}

	// Not due yet, so not picked.
	camps, err := c.NextCampaigns(ctx, nil, nil)
	if err != nil || len(camps) != 0 {
		t.Errorf("picked %d, %v", len(camps), err)
	}
}
