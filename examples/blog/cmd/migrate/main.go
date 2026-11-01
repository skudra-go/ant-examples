// Command migrate manages the database schema. Run it with no arguments for help.
// In the repository it uses the migrations folder, and in a deployed image, where
// that folder is absent, it uses the copy embedded in the binary.
package main

import (
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"

	"github.com/skudra-go/ant-examples/blog/migrations"
	"github.com/skudra-go/ant-examples/blog/models"
)

func main() {
	migrate.Main[models.AppDB](postgres.Dialect, migrate.Config{Embedded: migrations.Files})
}
