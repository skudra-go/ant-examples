package core_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/core"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

// upstreamSubscribers runs listmonk's query-subscribers and its count query.
func upstreamSubscribers(
	t *testing.T,
	db *models.AppDB,
	in core.SubscriberQuery,
	orderBy, order string,
	offset, limit int,
) ([]int32, int64) {
	t.Helper()

	cond := "TRUE"

	if in.Expr != "" {
		cond = in.Expr
	}

	listIDs := in.ListIDs
	if listIDs == nil {
		listIDs = []int32{}
	}

	text := strings.ReplaceAll(upstream(t, "query-subscribers"), "%query%", cond)
	text = strings.ReplaceAll(text, "%order%", "subscribers."+orderBy+" "+order)

	ids, err := ant.Raw[int32](
		db,
		"SELECT id FROM ("+strings.TrimSuffix(strings.TrimSpace(text), ";")+") q",
		listIDs,
		string(in.SubStatus),
		in.Search,
		offset,
		limit,
	).All(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	countText := strings.ReplaceAll(upstream(t, "query-subscribers-count"), "%query%", cond)

	total, err := ant.Raw[int64](
		db,
		countText,
		listIDs,
		string(in.SubStatus),
		in.Search,
	).First(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return ids, *total
}

func TestQuerySubscribersMatchesUpstream(t *testing.T) {
	c, db := setup(t)
	s := seed(t, c)
	ctx := t.Context()

	cases := []struct {
		name    string
		in      core.SubscriberQuery
		orderBy string
		order   string
		offset  int
		limit   int
	}{
		{"all", core.SubscriberQuery{}, "id", "asc", 0, 0},
		{"page", core.SubscriberQuery{}, "email", "desc", 2, 3},
		{"one list", core.SubscriberQuery{ListIDs: []int32{s.Lists[0].ID}}, "name", "asc", 0, 0},
		{
			"list and status",
			core.SubscriberQuery{
				ListIDs:   []int32{s.Lists[1].ID},
				SubStatus: models.SubscriptionUnconfirmed,
			},
			"id",
			"asc",
			0,
			0,
		},
		{"search", core.SubscriberQuery{Search: "user[0-2]"}, "id", "desc", 0, 0},
		{
			"expression on attribs",
			core.SubscriberQuery{Expr: `subscribers.attribs->>'city' = 'Riga'`},
			"created_at",
			"asc",
			0,
			0,
		},
		{
			"expression with jsonb ?",
			core.SubscriberQuery{Expr: `subscribers.attribs ? 'good'`},
			"id",
			"asc",
			0,
			0,
		},
		{
			"everything",
			core.SubscriberQuery{
				ListIDs: []int32{s.Lists[0].ID},
				Search:  "example",
				Expr:    `(subscribers.attribs->>'n')::int >= 2`,
			},
			"updated_at",
			"desc",
			0,
			1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, total, err := c.QuerySubscribers(
				ctx,
				tc.in,
				tc.orderBy,
				tc.order,
				tc.offset,
				tc.limit,
			)
			if err != nil {
				t.Fatal(err)
			}

			wantIDs, wantTotal := upstreamSubscribers(
				t,
				db,
				tc.in,
				tc.orderBy,
				tc.order,
				tc.offset,
				tc.limit,
			)

			gotIDs := make([]int32, len(got))

			for i, sub := range got {
				gotIDs[i] = sub.ID
			}

			if !slices.Equal(gotIDs, wantIDs) || total != wantTotal {
				t.Errorf("got %v total %d, upstream %v total %d", gotIDs, total, wantIDs, wantTotal)
			}
		})
	}
}

func TestQuerySubscribersRefusesWrites(t *testing.T) {
	// Not isolated: anttest.Isolate runs the test in a transaction, and a
	// nested Tx is a savepoint, which ignores ant.ReadOnly. Against the root,
	// the read-only transaction refuses the side effect.
	c := core.New(shared)

	_, _, err := c.QuerySubscribers(
		t.Context(),
		core.SubscriberQuery{Expr: "nextval('subscribers_id_seq') > 0"},
		"id",
		"asc",
		0,
		10,
	)
	if !errors.Is(err, core.ErrInvalid) || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("got %v, want a read-only error", err)
	}
}

func TestQuerySubscribersSortWhitelist(t *testing.T) {
	c, _ := setup(t)
	seed(t, c)

	// An unknown column falls back to id, and never reaches the SQL.
	got, _, err := c.QuerySubscribers(
		t.Context(),
		core.SubscriberQuery{},
		"id; DROP TABLE subscribers",
		"asc",
		0,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.IsSortedFunc(got, func(a, b *models.Subscriber) int { return int(a.ID - b.ID) }) {
		t.Error("not sorted by id")
	}
}

func TestInsertSubscriber(t *testing.T) {
	c, db := setup(t)
	s := seed(t, c)
	ctx := t.Context()

	// A blocklisted subscriber is unsubscribed from every list it is added to,
	// and a list ID that does not exist is skipped.
	sub := core.NewSubscriber("blocked@example.org", "Blocked", nil)
	sub.Status = models.SubscriberBlocklisted

	if err := c.InsertSubscriber(
		ctx,
		sub,
		[]int32{s.Lists[0].ID, 999999},
		models.SubscriptionConfirmed,
	); err != nil {
		t.Fatal(err)
	}

	got, err := c.GetSubscriber(ctx, 0, sub.UUID, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Subscriptions) != 1 ||
		got.Subscriptions[0].Status != models.SubscriptionUnsubscribed {
		t.Errorf("subscriptions: %+v", got.Subscriptions)
	}

	if got.Subscriptions[0].List == nil || got.Subscriptions[0].List.Name != "Weekly" {
		t.Errorf("list not loaded: %+v", got.Subscriptions[0].List)
	}

	// The same e-mail again is refused.
	again := core.NewSubscriber("blocked@example.org", "Again", nil)
	if err := c.InsertSubscriber(ctx, again, nil, ""); !errors.Is(err, core.ErrExists) {
		t.Errorf("got %v, want ErrExists", err)
	}

	_ = db
}

func TestUpsertSubscriber(t *testing.T) {
	c, db := setup(t)
	s := seed(t, c)
	ctx := t.Context()

	// Without overwrite, the name stays, and an existing subscription keeps its status.
	sub := core.NewSubscriber("user0@example.org", "Imported", map[string]any{"x": 1})
	if err := c.UpsertSubscriber(
		ctx,
		sub,
		[]int32{s.Lists[0].ID, s.Lists[2].ID},
		models.SubscriptionUnconfirmed,
		false,
	); err != nil {
		t.Fatal(err)
	}

	if sub.ID != s.Subscribers[0].ID {
		t.Fatalf("upsert returned id %d, want %d", sub.ID, s.Subscribers[0].ID)
	}

	got, err := c.GetSubscriber(ctx, sub.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "User A" {
		t.Errorf("name overwritten: %q", got.Name)
	}

	statuses := map[int32]models.SubscriptionStatus{}

	for _, sl := range got.Subscriptions {
		statuses[sl.ListID] = sl.Status
	}

	if statuses[s.Lists[0].ID] != models.SubscriptionConfirmed ||
		statuses[s.Lists[2].ID] != models.SubscriptionUnconfirmed {
		t.Errorf("statuses: %v", statuses)
	}

	// With overwrite, name, attributes and statuses change.
	sub = core.NewSubscriber("user0@example.org", "Imported", map[string]any{"x": 1})
	if err := c.UpsertSubscriber(
		ctx,
		sub,
		[]int32{s.Lists[0].ID},
		models.SubscriptionUnsubscribed,
		true,
	); err != nil {
		t.Fatal(err)
	}

	got, err = core.New(db).GetSubscriber(ctx, sub.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}

	// The context tracks the subscriber from the first read, so Reload rereads it.
	if err := db.Subscribers.Reload(ctx, got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "Imported" || got.Attribs.V["x"] != float64(1) {
		t.Errorf("not overwritten: %q %v", got.Name, got.Attribs.V)
	}
}

func TestUpdateSubscriberWithLists(t *testing.T) {
	c, _ := setup(t)
	s := seed(t, c)
	ctx := t.Context()
	sub := s.Subscribers[4] // in Weekly and Staff, confirmed

	got, err := c.UpdateSubscriberWithLists(
		ctx,
		sub.ID,
		core.SubscriberPatch{Name: "Renamed", Attribs: map[string]any{"city": "Kaunas"}},
		[]int32{s.Lists[0].ID, s.Lists[1].ID},
		models.SubscriptionUnconfirmed,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	byList := map[int32]models.SubscriptionStatus{}

	for _, sl := range got.Subscriptions {
		byList[sl.ListID] = sl.Status
	}

	want := map[int32]models.SubscriptionStatus{
		s.Lists[0].ID: models.SubscriptionConfirmed,   // kept: it was confirmed
		s.Lists[1].ID: models.SubscriptionUnconfirmed, // added
	}

	if len(byList) != 2 || byList[s.Lists[0].ID] != want[s.Lists[0].ID] ||
		byList[s.Lists[1].ID] != want[s.Lists[1].ID] {
		t.Errorf("got %v, want %v", byList, want)
	}

	// What the database holds, read fresh.
	n, err := ant.Raw[int64](
		c.DB(),
		`SELECT count(*) FROM subscriber_lists WHERE subscriber_id = ?`,
		sub.ID,
	).First(ctx)
	if err != nil || *n != 2 {
		t.Errorf("rows %v %v", n, err)
	}
}

func TestBlocklistAndOrphans(t *testing.T) {
	c, db := setup(t)
	s := seed(t, c)
	ctx := t.Context()

	n, err := c.BlocklistSubscribersByQuery(
		ctx,
		core.SubscriberQuery{Expr: `subscribers.attribs->>'city' = 'Riga'`},
	)
	if err != nil || n != 2 {
		t.Fatalf("blocklisted %d, %v", n, err)
	}

	left, err := ant.Raw[int64](
		db,
		`SELECT count(*) FROM subscriber_lists sl JOIN subscribers s ON s.id = sl.subscriber_id
		WHERE s.status = 'blocklisted' AND sl.status <> 'unsubscribed'`,
	).First(ctx)
	if err != nil || *left != 0 {
		t.Errorf("subscriptions not unsubscribed: %v %v", left, err)
	}

	// An orphan: a subscriber in no list.
	orphan := core.NewSubscriber("orphan@example.org", "Orphan", nil)
	if err := c.InsertSubscriber(ctx, orphan, nil, ""); err != nil {
		t.Fatal(err)
	}

	before, _ := ant.Raw[int64](
		db,
		`SELECT count(*) FROM subscribers a WHERE NOT EXISTS (SELECT 1 FROM subscriber_lists b WHERE b.subscriber_id = a.id)`,
	).First(ctx)

	deleted, err := c.DeleteOrphanSubscribers(ctx)
	if err != nil || deleted != *before || deleted < 1 {
		t.Errorf("deleted %d of %d orphans, %v", deleted, *before, err)
	}

	_ = s
}

func TestAddSubscribersToListsByQuery(t *testing.T) {
	c, db := setup(t)
	s := seed(t, c)
	ctx := t.Context()
	staff := s.Lists[2].ID

	// Riga subscribers into Staff; user4 is in Staff already and stays as is.
	n, err := c.AddSubscribersToListsByQuery(
		ctx,
		core.SubscriberQuery{Expr: `subscribers.attribs->>'city' IN ('Riga', 'Bengaluru')`},
		[]int32{staff},
		models.SubscriptionConfirmed,
	)
	if err != nil {
		t.Fatal(err)
	}

	// john and anon (install data, Bengaluru), user0 and user3 (Riga); user4 is
	// in Bengaluru and already in Staff.
	if n != 4 {
		t.Errorf("inserted %d, want 4", n)
	}

	members, err := ant.Pluck(
		ctx,
		db.SubscriberLists.Where(ant.Eq(&core.SL.ListID, staff)).OrderBy(&core.SL.SubscriberID),
		&core.SL.SubscriberID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(members) != 5 {
		t.Errorf("staff members %v", members)
	}
}
