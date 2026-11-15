package models

import (
	"context"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/ids"
)

// Header is one custom e-mail header of a campaign.
type Header map[string]string

// Campaign is a newsletter.
type Campaign struct {
	ID                int32                    `json:"id"`
	UUID              string                   `json:"uuid"`
	Name              string                   `json:"name"`
	Subject           string                   `json:"subject"`
	FromEmail         string                   `json:"from_email"`
	Body              string                   `json:"body"`
	BodySource        *string                  `json:"body_source"`
	AltBody           *string                  `json:"altbody"           ant:"column=altbody"`
	ContentType       ContentType              `json:"content_type"`
	SendAt            *time.Time               `json:"send_at"`
	Headers           ant.JSON[[]Header]       `json:"headers"`
	Status            CampaignStatus           `json:"status"`
	Tags              postgres.Array[string]   `json:"tags"`
	Type              *CampaignType            `json:"type"`
	Messenger         string                   `json:"messenger"`
	TemplateID        *int32                   `json:"template_id"`
	ToSend            int32                    `json:"to_send"`
	Sent              int32                    `json:"sent"`
	MaxSubscriberID   int32                    `json:"max_subscriber_id"`
	LastSubscriberID  int32                    `json:"last_subscriber_id"`
	Archive           bool                     `json:"archive"`
	ArchiveSlug       *string                  `json:"archive_slug"`
	ArchiveTemplateID *int32                   `json:"archive_template_id"`
	ArchiveMeta       ant.JSON[map[string]any] `json:"archive_meta"`
	StartedAt         *time.Time               `json:"started_at"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`

	Template        *Template      `json:"-"`
	ArchiveTemplate *Template      `json:"-"`
	Lists           []CampaignList `json:"lists,omitempty"`
}

// Campaigns is the probe.
var Campaigns = ant.Fields[Campaign]()

func (Campaign) Configure(b *ant.ModelBuilder[Campaign]) {
	b.Column(&Campaigns.UUID).Type("uuid").Unique()

	b.Column(&Campaigns.ContentType).
		Enum("content_type", "richtext", "html", "plain", "markdown", "visual").
		Default("'richtext'::content_type")

	b.Column(&Campaigns.Headers).Default("'[]'::jsonb")

	b.Column(&Campaigns.Status).
		Enum("campaign_status", "draft", "running", "scheduled", "paused", "cancelled", "finished").
		Default("'draft'::campaign_status")

	b.Column(&Campaigns.Tags).Type("character varying(100)[]").Nullable()

	b.Column(&Campaigns.Type).
		Enum("campaign_type", "regular", "optin").
		Default("'regular'::campaign_type")

	b.Column(&Campaigns.ToSend).Default("0")
	b.Column(&Campaigns.Sent).Default("0")
	b.Column(&Campaigns.MaxSubscriberID).Default("0")
	b.Column(&Campaigns.LastSubscriberID).Default("0")
	b.Column(&Campaigns.Archive).Default("false")
	b.Column(&Campaigns.ArchiveSlug).Unique()
	b.Column(&Campaigns.ArchiveMeta).Default("'{}'::jsonb")
	b.Column(&Campaigns.CreatedAt).Nullable().Default("now()")
	b.Column(&Campaigns.UpdatedAt).Nullable().Default("now()")

	b.BelongsTo(&Campaigns.Template).
		ForeignKey(&Campaigns.TemplateID).
		OnDelete(ant.SetNull).
		NoIndex()

	b.BelongsTo(&Campaigns.ArchiveTemplate).
		ForeignKey(&Campaigns.ArchiveTemplateID).
		OnDelete(ant.SetNull).
		NoIndex()

	b.Index(&Campaigns.Status).Name("idx_camps_status")
	b.Index(&Campaigns.Name).Name("idx_camps_name")
	b.Index(&Campaigns.CreatedAt).Name("idx_camps_created_at")
	b.Index(&Campaigns.UpdatedAt).Name("idx_camps_updated_at")
}

func (c *Campaign) BeforeInsert(context.Context) error {
	if c.UUID == "" {
		c.UUID = ids.UUID()
	}

	if c.Status == "" {
		c.Status = CampaignDraft
	}

	if c.ContentType == "" {
		c.ContentType = ContentRichtext
	}

	if c.Headers.V == nil {
		c.Headers.V = []Header{}
	}

	if c.ArchiveMeta.V == nil {
		c.ArchiveMeta.V = map[string]any{}
	}

	return nil
}

// CampaignList links a campaign to a list. The list may be deleted later, so
// list_id is nullable and the list's name is copied.
type CampaignList struct {
	ID         int64  `json:"-"`
	CampaignID int32  `json:"-"`
	ListID     *int32 `json:"id"`
	ListName   string `json:"name"`

	Campaign *Campaign `json:"-"`
	List     *List     `json:"-"`
}

// CampaignLists is the probe.
var CampaignLists = ant.Fields[CampaignList]()

func (CampaignList) Configure(b *ant.ModelBuilder[CampaignList]) {
	b.Column(&CampaignLists.ListName).Default("''::text")
	b.BelongsTo(&CampaignLists.Campaign).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&CampaignLists.List).OnDelete(ant.SetNull).OnUpdate(ant.Cascade).NoIndex()

	b.Index(&CampaignLists.CampaignID).
		And(&CampaignLists.ListID).
		Unique().
		Name("campaign_lists_campaign_id_list_id_idx")

	b.Index(&CampaignLists.CampaignID).Name("idx_camp_lists_camp_id")
	b.Index(&CampaignLists.ListID).Name("idx_camp_lists_list_id")
}

// CampaignView is one open of a campaign. A high-volume, append-only table.
type CampaignView struct {
	ID           int64
	CampaignID   int32
	SubscriberID *int32
	CreatedAt    time.Time

	Campaign   *Campaign
	Subscriber *Subscriber
}

// CampaignViews is the probe.
var CampaignViews = ant.Fields[CampaignView]()

func (CampaignView) Configure(b *ant.ModelBuilder[CampaignView]) {
	b.Column(&CampaignViews.CreatedAt).Nullable().Default("now()")
	b.BelongsTo(&CampaignViews.Campaign).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&CampaignViews.Subscriber).OnDelete(ant.SetNull).OnUpdate(ant.Cascade).NoIndex()

	b.Index(&CampaignViews.CampaignID).Name("idx_views_camp_id")
	b.Index(&CampaignViews.SubscriberID).Name("idx_views_subscriber_id")
	b.IndexExpr("((timezone('UTC'::text, created_at))::date)").Name("idx_views_date")
}
