package blog

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // the "pgx" database driver

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/blog/migrations"
	"github.com/skudra-go/ant-examples/blog/models"
)

// App is the running application. There is one, built in main and passed down,
// which keeps tests free to build their own.
type App struct {
	Config   Config
	Log      *slog.Logger
	DB       *models.AppDB
	Services Services
}

// New connects everything. Close it when done.
func New(ctx context.Context, config Config) (*App, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	// Tracking stays off on this shared context, because it would grow forever
	// and mix up requests. A service that wants SaveChanges takes an ant.Session.
	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		config.DatabaseURL,
		ant.WithTracking(false),
		ant.WithLogger(log),
	)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if config.AutoMigrate {
		applied := func(a migrate.Applied) {
			log.Info("migration applied", "id", a.Migration.ID(), "took", a.Duration)
		}

		err := migrate.Up(ctx, db, migrate.Config{FS: migrations.Files, OnApply: applied})
		if err != nil {
			return nil, fmt.Errorf("apply migrations: %w", err)
		}
	}

	return &App{
		Config:   config,
		Log:      log,
		DB:       db,
		Services: newServices(db),
	}, nil
}

// Close releases the database.
func (a *App) Close() error { return a.DB.Close() }
