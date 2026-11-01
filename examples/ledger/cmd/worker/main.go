// Command worker sends what the outbox holds. Run as many as you like: SKIP LOCKED
// gives each its own batch, so they share the work without waiting for each other.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/ledger/models"
	"github.com/skudra-go/ant-examples/ledger/services"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		os.Getenv("DATABASE_URL"),
		ant.WithTracking(false),
	)
	if err != nil {
		return err
	}
	defer db.Close()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// A real worker would post to a broker here. A failed publish rolls the batch
	// back, and it is sent again.
	publish := func(m *models.Message) error {
		log.Info("published", "id", m.ID, "topic", m.Topic, "tenant", m.TenantID)

		return nil
	}

	for {
		sent, err := services.Send(ctx, db, 50, publish)
		if err != nil && ctx.Err() == nil {
			log.Error("send failed", "error", err)
		}

		if sent == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}
