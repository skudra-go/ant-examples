// Command server runs the ledger API.
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

	ledger "github.com/skudra-go/ant-examples/ledger"
	"github.com/skudra-go/ant-examples/ledger/routes"
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

	config, err := ledger.LoadConfig()
	if err != nil {
		return err
	}

	app, err := ledger.New(ctx, config)
	if err != nil {
		return err
	}
	defer app.Close()

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
