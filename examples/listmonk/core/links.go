package core

import (
	"context"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/ids"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var (
	LK = models.Links
	LC = models.LinkClicks
)

// CreateLink returns the UUID of a tracked URL, inserting it the first time.
// It is listmonk's create-link: an upsert that returns the existing row.
func (c *Core) CreateLink(ctx context.Context, url string) (string, error) {
	// Upsert runs no insert hooks, so the UUID is made here.
	link := &models.Link{UUID: ids.UUID(), URL: url}

	_, err := c.db.Links.Upsert(ctx, link, ant.OnConflict(&LK.URL).DoUpdate(ant.F(&LK.URL)))

	return link.UUID, err
}

// RegisterLinkClick records a click and returns the URL to redirect to. The
// campaign and subscriber are optional: unknown UUIDs record NULL. listmonk
// does this in one INSERT with three subqueries; here it is three reads and an
// insert.
func (c *Core) RegisterLinkClick(
	ctx context.Context,
	linkUUID, campUUID, subUUID string,
) (string, error) {
	link, err := c.db.Links.NoTracking().Where(ant.Eq(&LK.UUID, linkUUID)).First(ctx)
	if err != nil {
		return "", notFound(err)
	}

	click := &models.LinkClick{LinkID: link.ID}

	if campUUID != "" {
		ids, err := ant.Pluck(ctx, c.db.Campaigns.Where(ant.Eq(&C.UUID, campUUID)), &C.ID)
		if err != nil {
			return "", err
		}

		if len(ids) > 0 {
			click.CampaignID = &ids[0]
		}
	}

	if subUUID != "" {
		ids, err := ant.Pluck(ctx, c.db.Subscribers.Where(ant.Eq(&S.UUID, subUUID)), &S.ID)
		if err != nil {
			return "", err
		}

		if len(ids) > 0 {
			click.SubscriberID = &ids[0]
		}
	}

	// InsertOnly: a click is append-only, and need not stay tracked.
	if err := c.db.LinkClicks.InsertOnly(ctx, click); err != nil {
		return "", err
	}

	return link.URL, nil
}
