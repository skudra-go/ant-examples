package migrations

import (
	"context"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/blog/models"
)

func init() {
	migrate.Register[models.AppDB]("0003_lowercase_tags", lowercaseTags)
}

// lowercaseTags folds tags like Go and GO into one. It runs in sequence with the
// SQL files, in a transaction of its own. It uses raw SQL and not models, because a
// model describes the latest schema, and this must keep working as models change.
func lowercaseTags(ctx context.Context, tx *models.AppDB) error {
	statements := []string{
		`INSERT INTO post_tags (post_id, tag_id)
		 SELECT pt.post_id, keeper.id
		 FROM post_tags pt
		 JOIN tags t ON t.id = pt.tag_id
		 JOIN LATERAL (SELECT min(id) AS id FROM tags WHERE lower(name) = lower(t.name)) keeper ON true
		 ON CONFLICT DO NOTHING`,
		`DELETE FROM tags WHERE id NOT IN (SELECT min(id) FROM tags GROUP BY lower(name))`,
		`UPDATE tags SET name = lower(name)`,
	}

	for _, statement := range statements {
		if _, err := ant.Raw[any](tx, statement).Exec(ctx); err != nil {
			return err
		}
	}

	return nil
}
