package core

import (
	"context"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var B = models.Bounces

// BounceAction is what happens to a subscriber at the bounce threshold.
type BounceAction string

const (
	BounceNone        BounceAction = "none"
	BounceUnsubscribe BounceAction = "unsubscribe"
	BounceBlocklist   BounceAction = "blocklist"
	BounceDelete      BounceAction = "delete"
)

// BounceInput is one bounce report.
type BounceInput struct {
	SubscriberUUID string
	Email          string
	CampaignUUID   string
	Type           models.BounceType
	Source         string
	Meta           map[string]any
	CreatedAt      time.Time
}

// RecordBounce records a bounce and applies the action once the subscriber's
// bounces of this type reach threshold. A blocklisted subscriber's bounces,
// and those past the threshold, are not recorded. It is listmonk's
// record-bounce: one statement of five CTEs there, a transaction here.
func (c *Core) RecordBounce(
	ctx context.Context,
	in BounceInput,
	threshold int64,
	action BounceAction,
) error {
	return c.db.Tx(ctx, func(tx *models.AppDB) error {
		cond := ant.Eq(&S.Email, in.Email)

		if in.SubscriberUUID != "" {
			cond = ant.Eq(&S.UUID, in.SubscriberUUID)
		}

		// Locked, so two reports for one subscriber count one after the other.
		sub, err := tx.Subscribers.NoTracking().Where(cond).ForUpdate().First(ctx)
		if err != nil {
			return notFound(err)
		}

		var campID *int32

		if in.CampaignUUID != "" {
			ids, err := ant.Pluck(ctx, tx.Campaigns.Where(ant.Eq(&C.UUID, in.CampaignUUID)), &C.ID)
			if err != nil {
				return err
			}

			if len(ids) > 0 {
				campID = &ids[0]
			}
		}

		count, err := tx.Bounces.Where(ant.Eq(&B.SubscriberID, sub.ID), ant.Eq(&B.Type, in.Type)).
			Count(ctx)
		if err != nil {
			return err
		}

		num := count + 1

		if sub.Status != models.SubscriberBlocklisted && num <= threshold {
			meta := in.Meta
			if meta == nil {
				meta = map[string]any{}
			}

			if err := tx.Bounces.InsertOnly(ctx, &models.Bounce{
				SubscriberID: sub.ID,
				CampaignID:   campID,
				Type:         in.Type,
				Source:       in.Source,
				Meta:         ant.JSON[map[string]any]{V: meta},
				CreatedAt:    in.CreatedAt,
			}); err != nil {
				return err
			}
		}

		if num < threshold {
			return nil
		}

		switch action {
		case BounceBlocklist:
			if sub.Status == models.SubscriberBlocklisted {
				return nil
			}

			_, err = tx.Subscribers.Where(ant.Eq(&S.ID, sub.ID)).
				Update(ctx, ant.Set(&S.Status, models.SubscriberBlocklisted))
		case BounceUnsubscribe:
			if sub.Status == models.SubscriberBlocklisted {
				return nil
			}

			_, err = tx.SubscriberLists.
				Where(ant.Eq(&SL.SubscriberID, sub.ID)).
				Update(ctx, ant.Set(&SL.Status, models.SubscriptionUnsubscribed))
		case BounceDelete:
			err = tx.Subscribers.DeleteByID(ctx, sub.ID)
		}

		return err
	})
}

// BounceRow is a bounce as the API lists it: with the subscriber's e-mail and
// the campaign's name.
type BounceRow struct {
	ID             int32                    `json:"id"`
	Type           models.BounceType        `json:"type"`
	Source         string                   `json:"source"`
	Meta           ant.JSON[map[string]any] `json:"meta"`
	CreatedAt      time.Time                `json:"created_at"`
	SubscriberID   int32                    `json:"subscriber_id"`
	SubscriberUUID string                   `json:"subscriber_uuid"`
	Email          string                   `json:"email"`
	CampaignID     *int32                   `json:"-"`
	CampaignName   *string                  `json:"-"`
}

var BR = ant.Fields[BounceRow]()

// QueryBounces lists bounces, newest first.
func (c *Core) QueryBounces(
	ctx context.Context,
	campID int32,
	offset, limit int,
) ([]BounceRow, error) {
	q := c.db.Bounces.Query().
		When(campID > 0, func(q *ant.Query[models.Bounce]) *ant.Query[models.Bounce] {
			return q.Where(ant.Eq(&B.CampaignID, &campID))
		}).
		Join(&B.Subscriber).
		LeftJoin(&B.Campaign).
		OrderByDesc(&B.CreatedAt).
		Offset(offset)

	if limit > 0 {
		q = q.Limit(limit)
	}

	return ant.Project[BounceRow](
		q,
		ant.Into(&BR.ID, &B.ID),
		ant.Into(&BR.Type, &B.Type),
		ant.Into(&BR.Source, &B.Source),
		ant.Into(&BR.Meta, &B.Meta),
		ant.Into(&BR.CreatedAt, &B.CreatedAt),
		ant.Into(&BR.SubscriberID, &B.SubscriberID),
		ant.Into(&BR.SubscriberUUID, &S.UUID),
		ant.Into(&BR.Email, &S.Email),
		ant.Into(&BR.CampaignID, &B.CampaignID),
		// The campaign is left-joined, so its name may be NULL, but the
		// column is a plain string: Into needs the same type on both sides.
		ant.IntoExpr(&BR.CampaignName, ant.SQL[*string]("?", ant.Col(&C.Name))),
	).All(ctx)
}
