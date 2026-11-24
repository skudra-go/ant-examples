package main

import (
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant-examples/playground/models"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"
)

func main() {
	migrate.Main[models.DB](postgres.Dialect)
}
