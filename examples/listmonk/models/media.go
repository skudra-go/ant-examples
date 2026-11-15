package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Media is an uploaded file.
type Media struct {
	ID          int32
	UUID        string
	Provider    string
	Filename    string
	ContentType string
	Thumb       string
	Meta        ant.JSON[map[string]any]
	CreatedAt   time.Time
}

// MediaFields is the probe.
var MediaFields = ant.Fields[Media]()

func (Media) Configure(b *ant.ModelBuilder[Media]) {
	b.Table("media")
	b.Column(&MediaFields.UUID).Type("uuid").Unique()
	b.Column(&MediaFields.Provider).Default("''::text")
	b.Column(&MediaFields.ContentType).Default("'application/octet-stream'::text")
	b.Column(&MediaFields.Meta).Default("'{}'::jsonb")
	b.Column(&MediaFields.CreatedAt).Nullable().Default("now()")
	b.Index(&MediaFields.Provider).And(&MediaFields.Filename).Name("idx_media_filename")
}

// CampaignMedia links a campaign to a media file. The table has no key.
type CampaignMedia struct {
	CampaignID *int32
	MediaID    *int32
	Filename   string

	Campaign *Campaign
	Media    *Media
}

// CampaignMediaFields is the probe.
var CampaignMediaFields = ant.Fields[CampaignMedia]()

func (CampaignMedia) Configure(b *ant.ModelBuilder[CampaignMedia]) {
	b.Table("campaign_media")
	b.NoKey()
	b.Column(&CampaignMediaFields.Filename).Default("''::text")
	b.BelongsTo(&CampaignMediaFields.Campaign).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.BelongsTo(&CampaignMediaFields.Media).OnDelete(ant.SetNull).OnUpdate(ant.Cascade).NoIndex()

	b.Index(&CampaignMediaFields.CampaignID).
		And(&CampaignMediaFields.MediaID).
		Unique().
		Name("idx_camp_media_id")

	b.Index(&CampaignMediaFields.CampaignID).Name("idx_camp_media_camp_id")
}
