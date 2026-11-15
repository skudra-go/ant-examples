package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Link is a tracked URL.
type Link struct {
	ID        int32
	UUID      string
	URL       string `ant:"unique"`
	CreatedAt time.Time
}

// Links is the probe.
var Links = ant.Fields[Link]()

func (Link) Configure(b *ant.ModelBuilder[Link]) {
	b.Column(&Links.UUID).Type("uuid").Unique()
	b.Column(&Links.CreatedAt).Nullable().Default("now()")
}

// LinkClick is one click on a tracked link. A high-volume, append-only table.
type LinkClick struct {
	ID           int64
	CampaignID   *int32
	LinkID       int32
	SubscriberID *int32
	CreatedAt    time.Time

	Campaign   *Campaign
	Link       *Link
	Subscriber *Subscriber
}

// LinkClicks is the probe.
var LinkClicks = ant.Fields[LinkClick]()

func (LinkClick) Configure(b *ant.ModelBuilder[LinkClick]) {
	b.Column(&LinkClicks.CreatedAt).Nullable().Default("now()")
	b.BelongsTo(&LinkClicks.Campaign).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&LinkClicks.Link).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&LinkClicks.Subscriber).OnDelete(ant.SetNull).OnUpdate(ant.Cascade).NoIndex()
	b.Index(&LinkClicks.CampaignID).Name("idx_clicks_camp_id")
	b.Index(&LinkClicks.LinkID).Name("idx_clicks_link_id")
	b.Index(&LinkClicks.SubscriberID).Name("idx_clicks_sub_id")
	b.IndexExpr("((timezone('UTC'::text, created_at))::date)").Name("idx_clicks_date")
}
