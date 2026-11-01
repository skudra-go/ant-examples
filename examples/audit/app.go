package audit

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // the "pgx" database driver

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/metrics"

	"github.com/skudra-go/ant-examples/audit/models"
	"github.com/skudra-go/ant-examples/audit/services"
)

// App is the running application.
type App struct {
	Config  Config
	Log     *slog.Logger
	DB      *models.AppDB
	Metrics *metrics.Metrics
	Ingest  *services.Ingest
	Search  *services.Search
	Persons *services.Persons
	Stats   *services.Stats
	Retain  *services.Retention
}

// New connects everything. Close it when done.
func New(ctx context.Context, config Config) (*App, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	counters := metrics.New()

	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		config.DatabaseURL,
		ant.WithTracking(false),
		ant.WithLogger(log, ant.SlowThreshold(250_000_000)),
		ant.WithInterceptor(counters),
		ant.WithDiagnostics(ant.Diagnostics{RepeatLimit: 50, Budget: 2_000_000_000}),
	)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	retention, err := services.NewRetention(db, log, config.RetentionMonths, config.PartitionsAhead)
	if err != nil {
		return nil, err
	}

	return &App{
		Config:  config,
		Log:     log,
		DB:      db,
		Metrics: counters,
		Ingest:  services.NewIngest(db),
		Search:  services.NewSearch(db),
		Persons: services.NewPersons(db),
		Stats:   services.NewStats(db),
		Retain:  retention,
	}, nil
}

// Close releases the database.
func (a *App) Close() error { return a.DB.Close() }
