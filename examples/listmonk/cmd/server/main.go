// Command server serves the ported slice of listmonk's API on DATABASE_URL, a
// database listmonk's own --install created.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"

	"github.com/skudra-go/ant-examples/listmonk/api"
	"github.com/skudra-go/ant-examples/listmonk/models"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// The root does no tracking and is shared; each request takes a session.
	db, err := ant.Open[models.AppDB](
		context.Background(),
		postgres.Dialect,
		os.Getenv("DATABASE_URL"),
		ant.WithTracking(false),
		ant.WithLogger(log, ant.SlowThreshold(250*time.Millisecond)),
	)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}

	defer db.Close()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "localhost:9100"
	}

	log.Info("listening", "addr", addr)

	server := &http.Server{
		Addr:              addr,
		Handler:           api.New(db, log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Error("serve", "error", err)
	}
}
