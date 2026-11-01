package ledger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // the "pgx" database driver

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/secure"

	"github.com/skudra-go/ant-examples/ledger/models"
	"github.com/skudra-go/ant-examples/ledger/services"
)

// App is the running application.
type App struct {
	Config    Config
	Log       *slog.Logger
	DB        *models.AppDB
	Accounts  *services.Accounts
	Journal   *services.Journal
	Reports   *services.Reports
	Reconcile *services.Reconcile
}

// New connects everything. Close it when done.
func New(ctx context.Context, config Config) (*App, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))

	if err := secure.SetKeys(
		"primary",
		map[string][]byte{"primary": config.EncryptionKey},
	); err != nil {
		return nil, err
	}

	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		config.DatabaseURL,
		ant.WithTracking(false),
		ant.WithLogger(log, ant.SlowThreshold(250*time.Millisecond)),
		ant.WithInterceptor(services.NewAuditTrail(log)),
		ant.WithDiagnostics(ant.Diagnostics{RepeatLimit: 25}),
	)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &App{
		Config:    config,
		Log:       log,
		DB:        db,
		Accounts:  services.NewAccounts(db),
		Journal:   services.NewJournal(db),
		Reports:   services.NewReports(db),
		Reconcile: services.NewReconcile(db),
	}, nil
}

// Close releases the database.
func (a *App) Close() error { return a.DB.Close() }
