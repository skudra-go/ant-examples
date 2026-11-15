// Command migrate checks the models against the Pagila database and manages its
// schema from here on. Run it with no arguments for help.
package main

import (
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/examples/pagila/models"
)

func main() {
	migrate.Main[models.AppDB](postgres.Dialect)
}
