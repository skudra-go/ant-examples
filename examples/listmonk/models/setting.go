package models

import (
	"encoding/json"
	"time"

	"github.com/skudra-go/ant"
)

// Setting is one key of listmonk's settings, with a JSON value of any shape.
// The table has no primary key, only a unique key column.
type Setting struct {
	Key       string                    `json:"key"`
	Value     ant.JSON[json.RawMessage] `json:"value"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

// Settings is the probe.
var Settings = ant.Fields[Setting]()

func (Setting) Configure(b *ant.ModelBuilder[Setting]) {
	b.NoKey()
	b.Column(&Settings.Key).Unique()
	b.Column(&Settings.Value).Default("'{}'::jsonb")
	b.Column(&Settings.UpdatedAt).Nullable().Default("now()")
	b.Index(&Settings.Key).Name("idx_settings_key")
}
