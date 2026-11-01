package migrations

import (
	"context"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/migrate"
	"github.com/skudra-go/ant/secure"

	"github.com/skudra-go/ant-examples/ledger/models"
)

func init() {
	migrate.Seed[models.AppDB]("demo", demo)
}

// demo adds a tenant with a small chart of accounts. Each row checks first, so
// running it again changes nothing. Accounts belong to a tenant, so the seeder sets
// one, as a request would.
func demo(ctx context.Context, db *models.AppDB) error {
	tenant, _, err := db.Tenants.FirstOrCreate(
		ctx,
		&models.Tenant{Name: "acme"},
		ant.Eq(&models.Tenants.Name, "acme"),
	)
	if err != nil {
		return err
	}
	// Hooks are muted while seeding, so the tenant is set by hand here.
	ctx = models.WithTenant(ctx, tenant.ID)

	chart := []struct {
		code, name, kind, parent string
		floor                    models.Money
	}{
		{"assets", "Assets", models.KindAsset, "", models.NoFloor},
		{"assets:bank", "Bank", models.KindAsset, "assets", 0},
		{"assets:savings", "Savings", models.KindAsset, "assets", 0},
		{"equity", "Equity", models.KindEquity, "", models.NoFloor},
		{"income", "Income", models.KindIncome, "", models.NoFloor},
		{"expenses", "Expenses", models.KindExpense, "", models.NoFloor},
	}

	created := map[string]*models.Account{}

	for _, row := range chart {
		account := &models.Account{
			Tenanted: models.Tenanted{TenantID: tenant.ID},
			Code:     row.code,
			Name:     row.name,
			Kind:     row.kind,
			Currency: "EUR",
			Floor:    row.floor,
			IBAN:     secure.New(""),
		}

		if parent := created[row.parent]; parent != nil {
			account.ParentID = &parent.ID
		}

		stored, _, err := db.Accounts.FirstOrCreate(
			ctx,
			account,
			ant.Eq(&models.Accounts.Code, row.code),
		)
		if err != nil {
			return err
		}

		created[row.code] = stored
	}

	return nil
}
