// Command migrate manages the schema from the models. Run it with no arguments for help.
package main

import (
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/listmonk/models"
)

func main() {
	migrate.Main[models.AppDB](postgres.Dialect)
}
