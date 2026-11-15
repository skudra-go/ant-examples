package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Template is an e-mail template.
type Template struct {
	ID         int32        `json:"id"`
	Name       string       `json:"name"`
	Type       TemplateType `json:"type"`
	Subject    string       `json:"subject"`
	Body       string       `json:"body"`
	BodySource *string      `json:"body_source"`
	IsDefault  bool         `json:"is_default"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// Templates is the probe.
var Templates = ant.Fields[Template]()

func (Template) Configure(b *ant.ModelBuilder[Template]) {

	b.Column(&Templates.Type).
		Enum("template_type", "campaign", "campaign_visual", "tx").
		Default("'campaign'::template_type")

	b.Column(&Templates.IsDefault).Default("false")
	b.Column(&Templates.CreatedAt).Nullable().Default("now()")
	b.Column(&Templates.UpdatedAt).Nullable().Default("now()")

	b.Index(&Templates.IsDefault).
		Unique().
		Where("is_default = true").
		Name("templates_is_default_idx")
}
