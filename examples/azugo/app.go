package app

import (
	"azugo.io/azugo"
	"azugo.io/azugo/server"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant-examples/playground/models"
	"github.com/skudra-go/ant/dialect/postgres"
)

type App struct {
	*azugo.App

	db       *models.DB
	config   *Config
	services *Services
}

func NewApp(cmd *cobra.Command) (*App, error) {
	config := NewConfig()

	app, err := server.New(cmd, server.Options{Configuration: config})
	if err != nil {
		return nil, err
	}

	app.AppName = config.App.Name

	app.SetVersion(
		config.App.Version,
		app.AppBuiltWith,
	)

	db, err := ant.Open[models.DB](
		app.BackgroundContext(),
		postgres.Dialect,
		config.Postgres.DSN(),
		ant.WithTracking(false),
	)
	if err != nil {
		return nil, err
	}

	return &App{
		App:      app,
		db:       db,
		config:   config,
		services: newServices(db),
	}, nil
}

func (a *App) Config() *Config {
	return a.config
}

func (a *App) Services() *Services {
	return a.services
}

func (a *App) Stop() {
	a.App.Stop()

	if err := a.db.Close(); err != nil {
		a.Log().Error("close database: " + err.Error())
	}
}
