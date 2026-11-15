package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Bounce is a bounced e-mail.
type Bounce struct {
	ID           int32                    `json:"id"`
	SubscriberID int32                    `json:"subscriber_id"`
	CampaignID   *int32                   `json:"campaign_id"`
	Type         BounceType               `json:"type"`
	Source       string                   `json:"source"`
	Meta         ant.JSON[map[string]any] `json:"meta"`
	CreatedAt    time.Time                `json:"created_at"`

	Subscriber *Subscriber `json:"-"`
	Campaign   *Campaign   `json:"-"`
}

// Bounces is the probe.
var Bounces = ant.Fields[Bounce]()

func (Bounce) Configure(b *ant.ModelBuilder[Bounce]) {

	b.Column(&Bounces.Type).
		Enum("bounce_type", "soft", "hard", "complaint").
		Default("'hard'::bounce_type")

	b.Column(&Bounces.Source).Default("''::text")
	b.Column(&Bounces.Meta).Default("'{}'::jsonb")
	b.Column(&Bounces.CreatedAt).Nullable().Default("now()")
	b.BelongsTo(&Bounces.Subscriber).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&Bounces.Campaign).OnDelete(ant.SetNull).OnUpdate(ant.Cascade).NoIndex()
	b.Index(&Bounces.SubscriberID).Name("idx_bounces_sub_id")
	b.Index(&Bounces.CampaignID).Name("idx_bounces_camp_id")
	b.Index(&Bounces.Source).Name("idx_bounces_source")
	b.IndexExpr("((timezone('UTC'::text, created_at))::date)").Name("idx_bounces_date")
}
