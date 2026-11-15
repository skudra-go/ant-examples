package models

import (
	"context"
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/ids"
)

// List is a mailing list.
type List struct {
	ID          int32                  `json:"id"`
	UUID        string                 `json:"uuid"`
	Name        string                 `json:"name"`
	Type        ListType               `json:"type"`
	Optin       ListOptin              `json:"optin"`
	Tags        postgres.Array[string] `json:"tags"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Lists is the probe.
var Lists = ant.Fields[List]()

func (List) Configure(b *ant.ModelBuilder[List]) {
	b.Column(&Lists.UUID).Type("uuid").Unique()
	b.Column(&Lists.Type).Enum("list_type", "public", "private", "temporary")
	b.Column(&Lists.Optin).Enum("list_optin", "single", "double").Default("'single'::list_optin")
	b.Column(&Lists.Tags).Type("character varying(100)[]").Nullable()
	b.Column(&Lists.Description).Default("''::text")
	b.Column(&Lists.CreatedAt).Nullable().Default("now()")
	b.Column(&Lists.UpdatedAt).Nullable().Default("now()")

	b.Index(&Lists.Type).Name("idx_lists_type")
	b.Index(&Lists.Optin).Name("idx_lists_optin")
	b.Index(&Lists.Name).Name("idx_lists_name")
	b.Index(&Lists.CreatedAt).Name("idx_lists_created_at")
	b.Index(&Lists.UpdatedAt).Name("idx_lists_updated_at")
}

func (l *List) BeforeInsert(context.Context) error {
	if l.UUID == "" {
		l.UUID = ids.UUID()
	}

	if l.Optin == "" {
		l.Optin = OptinSingle
	}

	return nil
}
