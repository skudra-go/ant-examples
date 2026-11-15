package core

import (
	"context"
	"encoding/json"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

var ST = models.Settings

// GetSettings returns every setting, by key. listmonk builds this map in SQL
// with JSON_OBJECT_AGG; here the rows are read and the map built in Go.
func (c *Core) GetSettings(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := c.db.Settings.NoTracking().OrderBy(&ST.Key).All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]json.RawMessage, len(rows))

	for _, r := range rows {
		out[r.Key] = r.Value.V
	}

	return out, nil
}

// UpdateSettings sets the value of each key that exists; unknown keys are
// ignored, as in listmonk's update-settings. The table has no primary key, so
// there is no UpdateMany or tracking: one UPDATE per key, in one transaction.
func (c *Core) UpdateSettings(
	ctx context.Context,
	values map[string]json.RawMessage,
) (int64, error) {
	var n int64

	err := c.db.Tx(ctx, func(tx *models.AppDB) error {
		for key, value := range values {
			changed, err := tx.Settings.
				Where(ant.Eq(&ST.Key, key)).
				Update(ctx, ant.Set(&ST.Value, ant.JSON[json.RawMessage]{V: value}))
			if err != nil {
				return err
			}

			n += changed
		}

		return nil
	})

	return n, err
}
