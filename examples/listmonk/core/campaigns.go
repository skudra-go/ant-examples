package core

import (
	"context"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var (
	C  = models.Campaigns
	CL = models.CampaignLists
	T  = models.Templates
)

var campaignSorts = Sorting{
	"name":       SortBy(&C.Name),
	"status":     SortBy(&C.Status),
	"created_at": SortBy(&C.CreatedAt),
	"updated_at": SortBy(&C.UpdatedAt),
}

// CampaignQuery filters the campaign list.
type CampaignQuery struct {
	Search   string
	Statuses []models.CampaignStatus
	Tags     []string
	ListIDs  []int32
}

// QueryCampaigns lists campaigns with their lists, a page at a time, with
// the total. It is listmonk's query-campaigns, where the lists come back as a
// JSON aggregate; here they are a loaded relation.
func (c *Core) QueryCampaigns(
	ctx context.Context,
	in CampaignQuery,
	orderBy, order string,
	offset, limit int,
) ([]*models.Campaign, int64, error) {
	q := c.db.Campaigns.NoTracking().
		When(len(in.Statuses) > 0, func(q *ant.Query[models.Campaign]) *ant.Query[models.Campaign] {
			return q.Where(ant.In(&C.Status, in.Statuses...))
		}).
		When(len(in.Tags) > 0, func(q *ant.Query[models.Campaign]) *ant.Query[models.Campaign] {
			// No typed array containment: a fragment, with the column and a bound array.
			return q.Where(ant.SQLCond("? @> ?", ant.Col(&C.Tags), postgres.Array[string](in.Tags)))
		}).
		When(in.Search != "", func(q *ant.Query[models.Campaign]) *ant.Query[models.Campaign] {
			text := ant.SQL[string]("CONCAT(?, ' ', ?)", ant.Col(&C.Name), ant.Col(&C.Subject))

			return q.Where(
				ant.Or(
					ant.SQLCond("TO_TSVECTOR(?) @@ PLAINTO_TSQUERY(?)", text, in.Search),
					ant.SQLCond("? ILIKE ?", text, "%"+in.Search+"%"),
				),
			)
		}).
		When(len(in.ListIDs) > 0, func(q *ant.Query[models.Campaign]) *ant.Query[models.Campaign] {
			return q.Where(ant.Has(&C.Lists, ant.In(&CL.ListID, pointers(in.ListIDs)...)))
		})

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	q = q.
		Order(campaignSorts.Term(orderBy, order, "created_at")).
		Include(&C.Lists, func(q *ant.Query[models.CampaignList]) *ant.Query[models.CampaignList] {
			return q.OrderBy(&CL.ID)
		}).
		Offset(offset)

	if limit > 0 {
		q = q.Limit(limit)
	}

	out, err := q.All(ctx)

	return out, total, err
}

// pointers turns IDs into pointers, for a nullable column.
func pointers[V any](values []V) []*V {
	out := make([]*V, len(values))

	for i := range values {
		out[i] = &values[i]
	}

	return out
}

// GetCampaign returns a campaign with its lists and the body of its template,
// or of the default template when it has none.
func (c *Core) GetCampaign(
	ctx context.Context,
	id int32,
	uuid string,
) (*models.Campaign, string, error) {
	cond := ant.Eq(&C.ID, id)

	if id == 0 {
		cond = ant.Eq(&C.UUID, uuid)
	}

	camp, err := c.db.Campaigns.
		Where(cond).
		IncludeOne(&C.Template).
		Include(&C.Lists).
		First(ctx)
	if err != nil {
		return nil, "", notFound(err)
	}

	if camp.Template != nil {
		return camp, camp.Template.Body, nil
	}

	def, err := c.db.Templates.NoTracking().Where(ant.Eq(&T.IsDefault, true)).First(ctx)
	if err != nil {
		return camp, "", ignoreNotFound(err)
	}

	return camp, def.Body, nil
}

func ignoreNotFound(err error) error {
	if err == nil || notFound(err) == ErrNotFound {
		return nil
	}

	return err
}

// audience is who a campaign goes to: in one of its lists, not blocklisted,
// and, per list, confirmed on a double opt-in list or not unsubscribed on a
// single opt-in one. An opt-in campaign goes to the unconfirmed of double
// opt-in lists.
func audience(campType models.CampaignType, lists []models.List) ant.Cond {
	var double, single []int32

	for _, l := range lists {
		if l.Optin == models.OptinDouble {
			double = append(double, l.ID)
		} else {
			single = append(single, l.ID)
		}
	}

	if campType == models.CampaignOptin {
		return ant.Has(
			&S.Subscriptions,
			ant.In(&SL.ListID, double...),
			ant.Eq(&SL.Status, models.SubscriptionUnconfirmed),
		)
	}

	return ant.Or(
		ant.Has(
			&S.Subscriptions,
			ant.In(&SL.ListID, double...),
			ant.Eq(&SL.Status, models.SubscriptionConfirmed),
		),
		ant.Has(
			&S.Subscriptions,
			ant.In(&SL.ListID, single...),
			ant.Ne(&SL.Status, models.SubscriptionUnsubscribed),
		),
	)
}

// audienceStats is the size of an audience and its highest subscriber ID.
type audienceStats struct {
	ToSend int64
	MaxID  int32
}

var audienceFields = ant.Fields[audienceStats]()

// countAudience counts a campaign's audience.
func (c *Core) countAudience(
	ctx context.Context,
	campType models.CampaignType,
	lists []models.List,
) (audienceStats, error) {
	if len(lists) == 0 {
		return audienceStats{}, nil
	}

	stats, err := ant.Project[audienceStats](
		c.db.Subscribers.Where(
			ant.Ne(&S.Status, models.SubscriberBlocklisted),
			audience(campType, lists),
		),
		ant.IntoExpr(&audienceFields.ToSend, ant.Count()),
		// MAX is NULL over no rows, and ant.Max is typed as never NULL.
		ant.IntoExpr(&audienceFields.MaxID, ant.SQL[int32]("COALESCE(MAX(?), 0)", ant.Col(&S.ID))),
	).First(ctx)
	if err != nil {
		return audienceStats{}, err
	}

	return *stats, nil
}

// CreateCampaign inserts a campaign and links it to the lists among listIDs,
// keeping a copy of each list's name. Without a template, the default one is
// used. It is listmonk's create-campaign, one CTE there, five statements here.
func (c *Core) CreateCampaign(ctx context.Context, camp *models.Campaign, listIDs []int32) error {
	return c.db.Tx(ctx, func(tx *models.AppDB) error {
		if camp.TemplateID == nil && camp.ContentType != models.ContentVisual {
			def, err := tx.Templates.NoTracking().Where(ant.Eq(&T.IsDefault, true)).First(ctx)
			if err := ignoreNotFound(err); err != nil {
				return err
			}

			if def != nil {
				camp.TemplateID = &def.ID
			}
		}

		lists, err := tx.Lists.NoTracking().Where(ant.In(&L.ID, listIDs...)).OrderBy(&L.ID).All(ctx)
		if err != nil {
			return err
		}

		if len(lists) == 0 {
			return ErrInvalid
		}

		values := make([]models.List, len(lists))

		for i, l := range lists {
			values[i] = *l
		}

		stats, err := c.With(tx).countAudience(ctx, models.CampaignRegular, values)
		if err != nil {
			return err
		}

		camp.ToSend = int32(stats.ToSend)
		camp.MaxSubscriberID = stats.MaxID

		for _, l := range lists {
			camp.Lists = append(camp.Lists, models.CampaignList{ListID: &l.ID, ListName: l.Name})
		}

		// Insert writes the campaign, then its lists with the new key.
		return tx.Campaigns.Insert(ctx, camp)
	})
}

// UpdateCampaignStatus sets a campaign's status. Starting a campaign that has
// a send time schedules it instead.
func (c *Core) UpdateCampaignStatus(
	ctx context.Context,
	id int32,
	status models.CampaignStatus,
) (*models.Campaign, error) {
	set := ant.Set(&C.Status, status)

	// ant casts the CASE to the enum type the column declares.
	if status == models.CampaignRunning {
		set = ant.SetExpr(&C.Status, ant.Case[models.CampaignStatus]().
			When(ant.IsNotNull(&C.SendAt), models.CampaignScheduled).
			Else(status))
	}

	rows, err := c.db.Campaigns.Where(ant.Eq(&C.ID, id)).UpdateReturning(ctx, set)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, ErrNotFound
	}

	return rows[0], nil
}

// NextCampaigns returns the campaigns to process now: running ones, and
// scheduled ones whose time has come, except those in exclude. Each is
// locked, marked running and given its audience's size and highest ID. Sent
// counts the caller reports are added first. It is listmonk's next-campaigns.
func (c *Core) NextCampaigns(
	ctx context.Context,
	exclude []int32,
	sent map[int32]int32,
) ([]*models.Campaign, error) {
	var out []*models.Campaign

	err := c.db.Tx(ctx, func(tx *models.AppDB) error {
		for id, n := range sent {
			if _, err := tx.Campaigns.Where(ant.Eq(&C.ID, id)).
				Update(ctx, ant.Inc(&C.Sent, n)); err != nil {
				return err
			}
		}

		now := time.Now()

		camps, err := tx.Campaigns.
			Where(
				ant.Or(
					ant.Eq(&C.Status, models.CampaignRunning),
					ant.And(
						ant.Eq(&C.Status, models.CampaignScheduled),
						// A nullable column has no ant.Lte: ant.Col compares it.
						ant.Col(&C.SendAt).Lte(&now),
					),
				),
				ant.NotIn(&C.ID, exclude...),
			).
			OrderBy(&C.ID).
			ForUpdate().
			SkipLocked().
			Include(&C.Lists, func(q *ant.Query[models.CampaignList]) *ant.Query[models.CampaignList] {
				return q.IncludeOne(&CL.List)
			}).
			All(ctx)
		if err != nil {
			return err
		}

		for _, camp := range camps {
			var lists []models.List

			for _, cl := range camp.Lists {
				if cl.List != nil {
					lists = append(lists, *cl.List)
				}
			}

			stats, err := c.With(tx).countAudience(ctx, campaignType(camp), lists)
			if err != nil {
				return err
			}

			camp.ToSend = int32(stats.ToSend)
			camp.MaxSubscriberID = stats.MaxID
			camp.Status = models.CampaignRunning

			if camp.StartedAt == nil {
				camp.StartedAt = &now
			}
		}

		out = camps

		return tx.SaveChanges(ctx)
	})

	return out, err
}

func campaignType(camp *models.Campaign) models.CampaignType {
	if camp.Type == nil {
		return models.CampaignRegular
	}

	return *camp.Type
}

// NextCampaignSubscribers returns the next batch of a running campaign's
// audience after its checkpoint, and moves the checkpoint past it. It is
// listmonk's get-running-campaign and next-campaign-subscribers.
func (c *Core) NextCampaignSubscribers(
	ctx context.Context,
	campID int32,
	limit int,
) ([]*models.Subscriber, error) {
	var out []*models.Subscriber

	err := c.db.Tx(ctx, func(tx *models.AppDB) error {
		camp, err := tx.Campaigns.
			Where(ant.Eq(&C.ID, campID), ant.Eq(&C.Status, models.CampaignRunning)).
			ForUpdate().
			Include(&C.Lists, func(q *ant.Query[models.CampaignList]) *ant.Query[models.CampaignList] {
				return q.IncludeOne(&CL.List)
			}).
			First(ctx)
		if err != nil {
			return notFound(err)
		}

		var lists []models.List

		for _, cl := range camp.Lists {
			if cl.List != nil {
				lists = append(lists, *cl.List)
			}
		}

		if len(lists) == 0 {
			return nil
		}

		out, err = tx.Subscribers.NoTracking().
			Where(
				ant.Gt(&S.ID, camp.LastSubscriberID),
				ant.Lte(&S.ID, camp.MaxSubscriberID),
				ant.Ne(&S.Status, models.SubscriberBlocklisted),
				audience(campaignType(camp), lists),
			).
			OrderBy(&S.ID).
			Limit(limit).
			All(ctx)
		if err != nil || len(out) == 0 {
			return err
		}

		camp.LastSubscriberID = out[len(out)-1].ID

		return tx.SaveChanges(ctx)
	})

	return out, err
}
