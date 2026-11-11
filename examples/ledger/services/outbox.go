package services

import (
	"context"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Send publishes a batch of messages from the outbox, and returns how many.
//
// FOR UPDATE SKIP LOCKED is what lets many workers share one table: each takes a
// batch no other worker holds, without waiting, so adding workers adds throughput. The
// rows stay locked until the commit. If a worker dies, its transaction rolls back,
// and the batch returns to the queue, so a message is sent at least once.
func Send(
	ctx context.Context,
	db *models.AppDB,
	size int,
	publish func(*models.Message) error,
) (int, error) {
	sent := 0
	err := db.Tx(ctx, func(tx *models.AppDB) error {
		batch, err := tx.Outbox.Where(ant.IsNull(&models.Messages.PublishedAt)).
			OrderBy(&models.Messages.ID).Limit(size).ForUpdate().SkipLocked().All(ctx)
		if err != nil {
			return err
		}

		ids := make([]int64, len(batch))

		for i, message := range batch {
			if err := publish(message); err != nil {
				return err
			}

			ids[i] = message.ID
		}

		sent = len(batch)
		if sent == 0 {
			return nil
		}

		now := time.Now().UTC()
		_, err = tx.Outbox.Where(ant.In(&models.Messages.ID, ids...)).
			Update(ctx, ant.Set(&models.Messages.PublishedAt, &now))

		return err
	})

	return sent, err
}
