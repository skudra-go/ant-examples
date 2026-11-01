// Command server runs the audit API, and the partition schedule.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	audit "github.com/skudra-go/ant-examples/audit"
	"github.com/skudra-go/ant-examples/audit/routes"
	"github.com/skudra-go/ant-examples/audit/schedule"
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

	config, err := audit.LoadConfig()
	if err != nil {
		return err
	}

	app, err := audit.New(ctx, config)
	if err != nil {
		return err
	}
	defer app.Close()

	// An event for a month with no partition has nowhere to go, so the months are
	// ready before the first request.
	if _, err := app.Retain.Prepare(ctx, time.Now(), config.PartitionsAhead); err != nil {
		return fmt.Errorf("prepare partitions: %w", err)
	}

	err = schedule.Start(ctx, app.Log, config.CleanupCron, func(ctx context.Context) error {
		_, err := app.Retain.Run(ctx, time.Now())

		return err
	})
	if err != nil {
		return fmt.Errorf("CLEANUP_CRON: %w", err)
	}

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           routes.New(app),
		ReadHeaderTimeout: 5 * time.Second,
	}

	failed := make(chan error, 1)

	go func() { failed <- server.ListenAndServe() }()

	app.Log.Info("listening", "port", config.Port)

	select {
	case err := <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	case <-ctx.Done():
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdown)
}
