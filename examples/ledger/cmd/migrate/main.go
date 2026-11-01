// Command migrate manages the database schema. Run it with no arguments for help.
package main

import (
	"encoding/base64"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"
	"github.com/skudra-go/ant/secure"

	"github.com/skudra-go/ant-examples/ledger/migrations"
	"github.com/skudra-go/ant-examples/ledger/models"
)

func main() {
	// Seeds write encrypted columns, so the key is set here too.
	if key, err := base64.StdEncoding.DecodeString(
		os.Getenv("ENCRYPTION_KEY"),
	); err == nil &&
		len(key) == 32 {
		_ = secure.SetKeys("primary", map[string][]byte{"primary": key})
	}

	migrate.Main[models.AppDB](postgres.Dialect, migrate.Config{Embedded: migrations.Files})
}
