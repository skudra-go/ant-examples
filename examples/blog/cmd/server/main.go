// Command server runs the blog API.
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

	blog "github.com/skudra-go/ant-examples/blog"
	"github.com/skudra-go/ant-examples/blog/routes"
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

	config, err := blog.LoadConfig()
	if err != nil {
		return err
	}

	app, err := blog.New(ctx, config)
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
	// Let requests in flight finish, but not forever.
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdown)
}
