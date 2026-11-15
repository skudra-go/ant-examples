package core

import (
	"context"
	"fmt"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var (
	S  = models.Subscribers
	SL = models.SubscriberLists
	L  = models.Lists
)

// subscriberSorts is the whitelist of columns the subscriber list sorts by.
var subscriberSorts = Sorting{
	"id":         SortBy(&S.ID),
	"email":      SortBy(&S.Email),
	"name":       SortBy(&S.Name),
	"status":     SortBy(&S.Status),
	"created_at": SortBy(&S.CreatedAt),
	"updated_at": SortBy(&S.UpdatedAt),
}

// withLists loads each subscriber's subscriptions and their lists.
func withLists(q *ant.Query[models.Subscriber]) *ant.Query[models.Subscriber] {
	return q.Include(
		&S.Subscriptions,
		func(q *ant.Query[models.SubscriberList]) *ant.Query[models.SubscriberList] {
			return q.IncludeOne(&SL.List).OrderBy(&SL.ListID)
		},
	)
}

// GetSubscriber returns one subscriber by ID, UUID or e-mail, the first one
// given, with its lists.
func (c *Core) GetSubscriber(
	ctx context.Context,
	id int32,
	uuid, email string,
) (*models.Subscriber, error) {
	var cond ant.Cond

	switch {
	case id > 0:
		cond = ant.Eq(&S.ID, id)
	case uuid != "":
		cond = ant.Eq(&S.UUID, uuid)
	case email != "":
		cond = ant.Eq(&S.Email, email)
	default:
		return nil, ErrInvalid
	}

	sub, err := withLists(c.db.Subscribers.Where(cond)).First(ctx)

	return sub, notFound(err)
}

// subscriptionStatus is the status a new subscription gets: a blocklisted
// subscriber is never subscribed.
func subscriptionStatus(
	sub models.SubscriberStatus,
	wanted models.SubscriptionStatus,
) models.SubscriptionStatus {
	if sub == models.SubscriberBlocklisted {
		return models.SubscriptionUnsubscribed
	}

	if wanted == "" {
		return models.SubscriptionUnconfirmed
	}

	return wanted
}

// InsertSubscriber inserts a subscriber and subscribes it to the lists that
// exist among listIDs. It is listmonk's insert-subscriber.
func (c *Core) InsertSubscriber(
	ctx context.Context,
	sub *models.Subscriber,
	listIDs []int32,
	status models.SubscriptionStatus,
) error {
	return c.db.Tx(ctx, func(tx *models.AppDB) error {
		if err := tx.Subscribers.InsertOnly(ctx, sub); err != nil {
			if ant.IsUniqueViolation(err) {
				return ErrExists
			}

			return err
		}

		return c.With(tx).
			subscribe(ctx, []int32{sub.ID}, listIDs, subscriptionStatus(sub.Status, status), true)
	})
}

// subscribe adds every subscriber to every list among listIDs that exists. On
// an existing subscription, overwrite sets the new status; otherwise the
// subscription is left as it is.
func (c *Core) subscribe(
	ctx context.Context,
	subIDs, listIDs []int32,
	status models.SubscriptionStatus,
	overwrite bool,
) error {
	if len(subIDs) == 0 || len(listIDs) == 0 {
		return nil
	}

	existing, err := ant.Pluck(ctx, c.db.Lists.Where(ant.In(&L.ID, listIDs...)), &L.ID)
	if err != nil {
		return err
	}

	rows := make([]*models.SubscriberList, 0, len(subIDs)*len(existing))

	for _, subID := range subIDs {
		for _, listID := range existing {
			rows = append(rows, &models.SubscriberList{
				SubscriberID: subID,
				ListID:       listID,
				Status:       status,
			})
		}
	}

	conflict := ant.OnConflict(&SL.SubscriberID).And(ant.F(&SL.ListID))

	if overwrite {
		conflict = conflict.DoUpdate(ant.F(&SL.Status))
	} else {
		conflict = conflict.DoNothing()
	}

	_, err = c.db.SubscriberLists.InsertMany(ctx, rows, ant.WithConflict(conflict))

	return err
}

// UpsertSubscriber inserts a subscriber or, when the e-mail exists, updates
// its name and attributes if overwrite is set. It then subscribes it to
// listIDs. It is listmonk's upsert-subscriber, used by the importer.
func (c *Core) UpsertSubscriber(
	ctx context.Context,
	sub *models.Subscriber,
	listIDs []int32,
	status models.SubscriptionStatus,
	overwrite bool,
) error {
	// Upsert runs no insert hooks, so the defaults BeforeInsert sets are set here.
	if err := sub.BeforeInsert(ctx); err != nil {
		return err
	}

	conflict := ant.OnConflict(&S.Email)

	if overwrite {
		conflict = conflict.DoUpdate(ant.F(&S.Name), ant.F(&S.Attribs))
	} else {
		// Touch the row so the upsert returns its ID and status.
		conflict = conflict.DoUpdate(ant.F(&S.UpdatedAt))
	}

	return c.db.Tx(ctx, func(tx *models.AppDB) error {
		if _, err := tx.Subscribers.Upsert(ctx, sub, conflict); err != nil {
			return err
		}

		return c.With(tx).
			subscribe(ctx, []int32{sub.ID}, listIDs, subscriptionStatus(sub.Status, status), overwrite)
	})
}

// SubscriberPatch holds the fields an update may change. Empty means keep.
type SubscriberPatch struct {
	Email   string
	Name    string
	Status  models.SubscriberStatus
	Attribs map[string]any
}

// UpdateSubscriberWithLists changes a subscriber and sets its lists: lists in
// listIDs are added, and when deleteOthers is set, lists not in it are
// removed. A confirmed subscription keeps its status. It is listmonk's
// update-subscriber-with-lists, as change tracking.
func (c *Core) UpdateSubscriberWithLists(
	ctx context.Context,
	id int32,
	patch SubscriberPatch,
	listIDs []int32,
	status models.SubscriptionStatus,
	deleteOthers bool,
) (*models.Subscriber, error) {
	var out *models.Subscriber

	err := c.db.Tx(ctx, func(tx *models.AppDB) error {
		sub, err := withLists(tx.Subscribers.Where(ant.Eq(&S.ID, id))).First(ctx)
		if err != nil {
			return notFound(err)
		}

		if patch.Email != "" {
			sub.Email = patch.Email
		}

		if patch.Name != "" {
			sub.Name = patch.Name
		}

		if patch.Status != "" {
			sub.Status = patch.Status
		}

		if patch.Attribs != nil {
			sub.Attribs.V = patch.Attribs
		}

		wanted := map[int32]bool{}

		existing, err := ant.Pluck(ctx, tx.Lists.Where(ant.In(&L.ID, listIDs...)), &L.ID)
		if err != nil {
			return err
		}

		for _, listID := range existing {
			wanted[listID] = true
		}

		// The loaded relation is the truth: dropping an element deletes that
		// subscription, appending one inserts it.
		kept := sub.Subscriptions[:0]

		for _, s := range sub.Subscriptions {
			switch {
			case wanted[s.ListID]:
				delete(wanted, s.ListID)

				if sub.Status == models.SubscriberBlocklisted {
					s.Status = models.SubscriptionUnsubscribed
				} else if s.Status != models.SubscriptionConfirmed {
					s.Status = subscriptionStatus(sub.Status, status)
				}

				kept = append(kept, s)
			case deleteOthers:
				// dropped
			default:
				kept = append(kept, s)
			}
		}

		for _, listID := range existing {
			if wanted[listID] {
				kept = append(kept, models.SubscriberList{
					SubscriberID: sub.ID,
					ListID:       listID,
					Status:       subscriptionStatus(sub.Status, status),
				})
			}
		}

		sub.Subscriptions = kept

		if err := tx.SaveChanges(ctx); err != nil {
			if ant.IsUniqueViolation(err) {
				return ErrExists
			}

			return err
		}

		out = sub

		return nil
	})

	return out, err
}

// SubscriberQuery is the filter of the subscriber list and of the bulk
// actions "by query".
type SubscriberQuery struct {
	// Search matches name or e-mail, as a case-insensitive regular expression.
	Search string

	// Expr is an SQL condition typed by an admin, as listmonk's advanced
	// query. It is SQL by design, and runs only in a read-only transaction
	// when listing.
	Expr string

	// ListIDs keeps subscribers of any of these lists; empty means all.
	ListIDs []int32

	// SubStatus keeps only subscriptions with this status, with ListIDs.
	SubStatus models.SubscriptionStatus
}

// query builds the filter.
func (c *Core) subscriberQuery(db *models.AppDB, in SubscriberQuery) *ant.Query[models.Subscriber] {
	return db.Subscribers.NoTracking().
		When(
			len(in.ListIDs) > 0,
			func(q *ant.Query[models.Subscriber]) *ant.Query[models.Subscriber] {
				conds := []ant.Cond{ant.In(&SL.ListID, in.ListIDs...)}

				if in.SubStatus != "" {
					conds = append(conds, ant.Eq(&SL.Status, in.SubStatus))
				}

				return q.Where(ant.Has(&S.Subscriptions, conds...))
			},
		).
		When(
			in.Search != "",
			func(q *ant.Query[models.Subscriber]) *ant.Query[models.Subscriber] {
				return q.Where(
					ant.Or(
						ant.SQLCond("? ~* ?", ant.Col(&S.Name), in.Search),
						ant.SQLCond("? ~* ?", ant.Col(&S.Email), in.Search),
					),
				)
			},
		).
		When(
			in.Expr != "",
			func(q *ant.Query[models.Subscriber]) *ant.Query[models.Subscriber] {
				return q.Where(ant.SQLCond(in.Expr))
			},
		)
}

// QuerySubscribers lists subscribers matching in, sorted by a whitelisted
// column, one page at a time, with the total. A limit below 1 returns all.
func (c *Core) QuerySubscribers(
	ctx context.Context,
	in SubscriberQuery,
	orderBy, order string,
	offset, limit int,
) ([]*models.Subscriber, int64, error) {
	var (
		out   []*models.Subscriber
		total int64
	)

	// The admin's expression is SQL, so it runs read-only.
	err := c.db.Tx(
		ctx,
		func(tx *models.AppDB) error {
			q := c.subscriberQuery(tx, in)

			var err error

			total, err = q.Count(ctx)
			if err != nil || total == 0 {
				return err
			}

			q = q.Order(subscriberSorts.Term(orderBy, order, "id")).Offset(offset)

			if limit > 0 {
				q = q.Limit(limit)
			}

			out, err = withLists(q).All(ctx)

			return err
		},
		ant.ReadOnly(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	if out == nil {
		out = []*models.Subscriber{}
	}

	return out, total, nil
}

// BlocklistSubscribers blocklists subscribers and unsubscribes them from all
// their lists.
func (c *Core) BlocklistSubscribers(ctx context.Context, subIDs []int32) error {
	return c.db.Tx(ctx, func(tx *models.AppDB) error {
		if _, err := tx.Subscribers.
			Where(ant.In(&S.ID, subIDs...)).
			Update(ctx, ant.Set(&S.Status, models.SubscriberBlocklisted)); err != nil {
			return err
		}

		_, err := tx.SubscriberLists.
			Where(ant.In(&SL.SubscriberID, subIDs...)).
			Update(ctx, ant.Set(&SL.Status, models.SubscriptionUnsubscribed))

		return err
	})
}

// BlocklistSubscribersByQuery is BlocklistSubscribers for every subscriber the
// query matches.
func (c *Core) BlocklistSubscribersByQuery(ctx context.Context, in SubscriberQuery) (int64, error) {
	var n int64

	err := c.db.Tx(ctx, func(tx *models.AppDB) error {
		ids := ant.SubOf(c.subscriberQuery(tx, in), &S.ID)

		var err error

		n, err = tx.Subscribers.
			Where(ant.InSub(&S.ID, ids)).
			Update(ctx, ant.Set(&S.Status, models.SubscriberBlocklisted))
		if err != nil {
			return err
		}

		_, err = tx.SubscriberLists.
			Where(ant.InSub(&SL.SubscriberID, ids)).
			Update(ctx, ant.Set(&SL.Status, models.SubscriptionUnsubscribed))

		return err
	})

	return n, err
}

// DeleteSubscribersByQuery deletes every subscriber the query matches.
func (c *Core) DeleteSubscribersByQuery(ctx context.Context, in SubscriberQuery) (int64, error) {
	return c.db.Subscribers.
		Where(ant.InSub(&S.ID, ant.SubOf(c.subscriberQuery(c.db, in), &S.ID))).
		Delete(ctx)
}

// AddSubscribersToLists subscribes subscribers to lists. An existing
// subscription gets status when one is given, and is kept otherwise.
func (c *Core) AddSubscribersToLists(
	ctx context.Context,
	subIDs, listIDs []int32,
	status models.SubscriptionStatus,
) error {
	return c.db.Tx(ctx, func(tx *models.AppDB) error {
		return c.With(tx).
			subscribe(ctx, subIDs, listIDs, subscriptionStatus("", status), status != "")
	})
}

// AddSubscribersToListsByQuery subscribes every subscriber the query matches to
// listIDs, in one INSERT ... SELECT. Existing subscriptions are left alone.
func (c *Core) AddSubscribersToListsByQuery(
	ctx context.Context,
	in SubscriberQuery,
	listIDs []int32,
	status models.SubscriptionStatus,
) (int64, error) {
	if status == "" {
		status = models.SubscriptionUnconfirmed
	}

	// A cross join of the matching subscribers and the target lists, minus the
	// pairs that already exist. InsertFrom takes no conflict clause.
	pairs := c.subscriberQuery(c.db, in).
		CrossJoin[models.List]().
		Where(
			ant.In(&L.ID, listIDs...),
			ant.NotExists(
				c.db.SubscriberLists.Where(
					ant.EqCol(&SL.SubscriberID, &S.ID),
					ant.EqCol(&SL.ListID, &L.ID),
				),
			),
		)

	return c.db.SubscriberLists.InsertFrom(
		ctx,
		ant.Project[models.SubscriberList](
			pairs,
			ant.Into(&SL.SubscriberID, &S.ID),
			ant.Into(&SL.ListID, &L.ID),
			ant.IntoExpr(
				&SL.Status,
				ant.SQL[models.SubscriptionStatus]("?::subscription_status", string(status)),
			),
		),
	)
}

// DeleteOrphanSubscribers deletes subscribers that belong to no list.
func (c *Core) DeleteOrphanSubscribers(ctx context.Context) (int64, error) {
	return c.db.Subscribers.Where(ant.HasNone(&S.Subscriptions)).Delete(ctx)
}

// NewSubscriber returns a subscriber with its UUID set, for callers that need
// it before the insert.
func NewSubscriber(email, name string, attribs map[string]any) *models.Subscriber {
	if attribs == nil {
		attribs = map[string]any{}
	}

	return &models.Subscriber{
		UUID:    ids.UUID(),
		Email:   email,
		Name:    name,
		Status:  models.SubscriberEnabled,
		Attribs: ant.JSON[map[string]any]{V: attribs},
	}
}
