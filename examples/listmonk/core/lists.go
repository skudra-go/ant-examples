package core

import (
	"context"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var LS = models.ListStats

// ListWithCounts is a list with its subscriber counts, from the
// mat_list_subscriber_stats materialized view.
type ListWithCounts struct {
	models.List

	SubscriberCount    int64                               `json:"subscriber_count"`
	SubscriberStatuses map[models.SubscriptionStatus]int64 `json:"subscriber_statuses"`
}

// ListQuery filters the lists.
type ListQuery struct {
	Search string
	Type   models.ListType
	Optin  models.ListOptin
	Tags   []string
}

// subscriberCount is the count of a list's subscribers in the view, as an
// expression: a correlated subquery. SUM is NULL over no rows, so COALESCE.
func subscriberCount(db *models.AppDB) ant.Expr[int64] {
	return ant.SQL[int64](
		"COALESCE((?), 0)::bigint",
		ant.SubExpr(
			db.ListStats.Where(ant.EqCol(&LS.ListID, &L.ID)),
			ant.SQL[int64]("SUM(?)", ant.Col(&LS.SubscriberCount)),
		).Expr(),
	)
}

// QueryLists lists lists with their subscriber counts, sorted by a
// whitelisted column, subscriber_count included. It is listmonk's query-lists.
func (c *Core) QueryLists(
	ctx context.Context,
	in ListQuery,
	orderBy, order string,
	offset, limit int,
) ([]ListWithCounts, int64, error) {
	count := subscriberCount(c.db)

	sorts := Sorting{
		"id":         SortBy(&L.ID),
		"name":       SortBy(&L.Name),
		"created_at": SortBy(&L.CreatedAt),
		"updated_at": SortBy(&L.UpdatedAt),
		// Not a column: the sort is an expression.
		"subscriber_count": func(desc bool) ant.Sort {
			if desc {
				return count.Desc()
			}

			return count.Asc()
		},
	}

	q := c.db.Lists.NoTracking().
		When(in.Search != "", func(q *ant.Query[models.List]) *ant.Query[models.List] {
			return q.Where(
				ant.SQLCond("to_tsvector(?) @@ to_tsquery(?)", ant.Col(&L.Name), in.Search),
			)
		}).
		When(in.Type != "", func(q *ant.Query[models.List]) *ant.Query[models.List] {
			return q.Where(ant.Eq(&L.Type, in.Type))
		}).
		When(in.Optin != "", func(q *ant.Query[models.List]) *ant.Query[models.List] {
			return q.Where(ant.Eq(&L.Optin, in.Optin))
		}).
		When(len(in.Tags) > 0, func(q *ant.Query[models.List]) *ant.Query[models.List] {
			return q.Where(ant.SQLCond("? <@ ?", postgres.Array[string](in.Tags), ant.Col(&L.Tags)))
		})

	total, err := q.Count(ctx)
	if err != nil || total == 0 {
		return []ListWithCounts{}, total, err
	}

	q = q.Order(sorts.Term(orderBy, order, "created_at"), ant.Col(&L.ID).Asc()).Offset(offset)

	if limit > 0 {
		q = q.Limit(limit)
	}

	lists, err := q.All(ctx)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]int32, len(lists))

	for i, l := range lists {
		ids[i] = l.ID
	}

	stats, err := c.db.ListStats.NoTracking().
		Where(ant.In(&LS.ListID, ids...), ant.IsNotNull(&LS.Status)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]ListWithCounts, len(lists))
	byID := map[int32]*ListWithCounts{}

	for i, l := range lists {
		out[i] = ListWithCounts{List: *l, SubscriberStatuses: map[models.SubscriptionStatus]int64{}}
		byID[l.ID] = &out[i]
	}

	for _, s := range stats {
		row := byID[s.ListID]
		row.SubscriberStatuses[*s.Status] = s.SubscriberCount
		row.SubscriberCount += s.SubscriberCount
	}

	return out, total, nil
}

// CreateList inserts a list.
func (c *Core) CreateList(ctx context.Context, l *models.List) error {
	return c.db.Lists.Insert(ctx, l)
}

// ListPatch holds the fields an update may change. Empty means keep, except
// Tags, which is always written, as listmonk does.
type ListPatch struct {
	Name        string
	Type        models.ListType
	Optin       models.ListOptin
	Tags        []string
	Description string
}

// UpdateList changes a list, with change tracking: only changed columns are written.
func (c *Core) UpdateList(ctx context.Context, id int32, patch ListPatch) (*models.List, error) {
	l, err := c.db.Lists.Find(ctx, id)
	if err != nil {
		return nil, notFound(err)
	}

	if patch.Name != "" {
		l.Name = patch.Name
	}

	if patch.Type != "" {
		l.Type = patch.Type
	}

	if patch.Optin != "" {
		l.Optin = patch.Optin
	}

	if patch.Description != "" {
		l.Description = patch.Description
	}

	l.Tags = patch.Tags

	return l, c.db.SaveChanges(ctx)
}

// RefreshListStats refreshes the materialized view behind the counts. ant has
// no notion of materialized views, so this is raw SQL.
func (c *Core) RefreshListStats(ctx context.Context) error {
	_, err := ant.Raw[any](
		c.db,
		`REFRESH MATERIALIZED VIEW CONCURRENTLY mat_list_subscriber_stats`,
	).Exec(ctx)

	return err
}
