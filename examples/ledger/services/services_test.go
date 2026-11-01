package services_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/anttest"
	"github.com/skudra-go/ant/dialect/postgres"
	"github.com/skudra-go/ant/migrate"
	"github.com/skudra-go/ant/secure"

	"github.com/skudra-go/ant-examples/ledger/migrations"
	"github.com/skudra-go/ant-examples/ledger/models"
	"github.com/skudra-go/ant-examples/ledger/services"
)

// accountsFactory makes valid accounts, so a test states only what it cares about.
var accountsFactory = anttest.NewFactory(func(n int) *models.Account {
	return &models.Account{
		Code:     fmt.Sprintf("account-%d", n),
		Name:     "Account",
		Kind:     models.KindAsset,
		Currency: "EUR",
		Floor:    models.NoFloor,
		IBAN:     secure.New(""),
	}
})

// open returns a database built by the real migrations, and a context for a tenant.
func open(t *testing.T) (*models.AppDB, context.Context) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	if err := secure.SetKeys(
		"test",
		map[string][]byte{"test": bytes.Repeat([]byte{7}, 32)},
	); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()

	admin, err := ant.Open[models.AppDB](ctx, postgres.Dialect, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	const schema = "test_ledger"
	if _, err := admin.DB().
		ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE; CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}

	db, err := ant.Open[models.AppDB](
		ctx,
		postgres.Dialect,
		dsn+"&search_path="+schema,
		ant.WithTracking(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = db.DB().ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		db.Close()
	})

	if err := migrate.Up(ctx, db, migrate.Config{FS: migrations.Files}); err != nil {
		t.Fatal(err)
	}

	tenant := &models.Tenant{Name: "acme"}
	if err := db.Tenants.Insert(ctx, tenant); err != nil {
		t.Fatal(err)
	}

	return db, models.WithTenant(ctx, tenant.ID)
}

func account(
	t *testing.T,
	ctx context.Context,
	db *models.AppDB,
	code string,
	floor models.Money,
) *models.Account {
	t.Helper()

	a := accountsFactory.Build(func(a *models.Account) { a.Code, a.Floor = code, floor })
	if err := db.Accounts.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}

	return a
}

func balance(t *testing.T, ctx context.Context, db *models.AppDB, code string) models.Money {
	t.Helper()

	a, err := db.Accounts.Where(ant.Eq(&models.Accounts.Code, code)).First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return a.Balance
}

func TestTransfer(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	account(t, ctx, db, "equity", models.NoFloor)
	account(t, ctx, db, "bank", 0)

	if _, err := journal.Post(
		ctx,
		services.Transfer("equity", "bank", 10000, "fund", ""),
	); err != nil {
		t.Fatal(err)
	}

	if got := balance(t, ctx, db, "bank"); got != 10000 {
		t.Fatalf("bank: %s", got)
	}

	t.Run("the floor stops an overdraft, in the database", func(t *testing.T) {
		_, err := journal.Post(ctx, services.Transfer("bank", "equity", 10001, "too much", ""))
		if !errors.Is(err, services.ErrInsufficientFunds) {
			t.Fatalf("got %v", err)
		}

		if got := balance(t, ctx, db, "bank"); got != 10000 {
			t.Fatalf("a refused transfer moved money: %s", got)
		}

		lines, _ := db.Lines.Count(ctx)
		if lines != 2 {
			t.Fatalf("a refused transfer left %d lines", lines)
		}
	})

	t.Run("a key makes a repeat harmless", func(t *testing.T) {
		first, err := journal.Post(ctx, services.Transfer("bank", "equity", 100, "once", "k1"))
		if err != nil {
			t.Fatal(err)
		}

		again, err := journal.Post(ctx, services.Transfer("bank", "equity", 100, "once", "k1"))
		if err != nil {
			t.Fatal(err)
		}

		if first.ID != again.ID || balance(t, ctx, db, "bank") != 9900 {
			t.Fatalf("the repeat posted twice: %d, %d", first.ID, again.ID)
		}
	})

	t.Run("an entry cannot mix currencies", func(t *testing.T) {
		usd := accountsFactory.Build(func(a *models.Account) { a.Code, a.Currency = "usd", "USD" })
		if err := db.Accounts.Insert(ctx, usd); err != nil {
			t.Fatal(err)
		}

		_, err := journal.Post(ctx, services.Transfer("bank", "usd", 1, "mixed", ""))
		if !errors.Is(err, services.ErrInvalid) {
			t.Fatalf("got %v", err)
		}
	})
}

// Fifty transfers race over accounts with little money. However they interleave,
// no account may pass its floor, and the books must balance to the cent.
func TestConcurrentTransfersKeepTheBooks(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	account(t, ctx, db, "equity", models.NoFloor)

	codes := []string{"a", "b", "c", "d"}
	for _, code := range codes {
		account(t, ctx, db, code, 0)

		if _, err := journal.Post(
			ctx,
			services.Transfer("equity", code, 1000, "fund", ""),
		); err != nil {
			t.Fatal(err)
		}
	}

	var (
		wg             sync.WaitGroup
		moved, refused atomic.Int64
	)

	for i := range 50 {
		wg.Go(func() {
			from, to := codes[i%4], codes[(i+1+i%3)%4]
			if from == to {
				to = codes[(i+2)%4]
			}

			_, err := journal.Post(ctx, services.Transfer(from, to, 300, "race", ""))

			switch {
			case err == nil:
				moved.Add(1)
			case errors.Is(err, services.ErrInsufficientFunds):
				refused.Add(1)
			default:
				t.Errorf("transfer: %v", err)
			}
		})
	}

	wg.Wait()

	var total models.Money

	for _, code := range codes {
		got := balance(t, ctx, db, code)
		if got < 0 {
			t.Fatalf("%s is overdrawn: %s", code, got)
		}

		total += got
	}

	if total != 4000 {
		t.Fatalf("money was created or lost: %s", total)
	}

	report, err := services.NewReports(db).Trial(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if !report.Balanced {
		t.Fatalf("the trial balance is off by %s", report.Total)
	}

	t.Logf("%d moved, %d refused", moved.Load(), refused.Load())
}

func TestDatabaseEnforcesTheRules(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	a := account(t, ctx, db, "a", models.NoFloor)
	account(t, ctx, db, "b", models.NoFloor)

	if _, err := journal.Post(ctx, services.Transfer("a", "b", 100, "fine", "")); err != nil {
		t.Fatal(err)
	}

	t.Run("an unbalanced entry fails at commit, even past Go", func(t *testing.T) {
		err := db.Tx(ctx, func(tx *models.AppDB) error {
			if _, err := ant.Raw[any](
				tx,
				"INSERT INTO entries (tenant_id, memo, created_at) VALUES (?, 'bad', now())",
				a.TenantID,
			).Exec(ctx); err != nil {
				return err
			}

			_, err := ant.Raw[any](
				tx,
				`INSERT INTO lines (tenant_id, entry_id, account_id, amount, created_at)
				SELECT ?, max(id), ?, 5, now() FROM entries`,
				a.TenantID,
				a.ID,
			).Exec(ctx)

			return err
		})
		if !ant.IsCheckViolation(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("a posted line cannot change", func(t *testing.T) {
		_, err := ant.Raw[any](db, "UPDATE lines SET amount = 1").Exec(ctx)
		if err == nil {
			t.Fatal("an update got through")
		}

		_, err = db.Lines.Where(ant.Gt(&models.Lines.ID, 0)).Delete(ctx)
		if err == nil {
			t.Fatal("a delete got through")
		}
	})
}

func TestTenantsCannotSeeEachOther(t *testing.T) {
	db, acme := open(t)

	other := &models.Tenant{Name: "other"}
	if err := db.Tenants.Insert(acme, other); err != nil {
		t.Fatal(err)
	}

	theirs := models.WithTenant(context.Background(), other.ID)

	account(t, acme, db, "shared-code", models.NoFloor)
	account(t, theirs, db, "shared-code", models.NoFloor) // the same code is fine per tenant

	count, err := db.Accounts.Count(theirs)
	if err != nil || count != 1 {
		t.Fatalf("other tenant sees %d, %v", count, err)
	}

	t.Run("a query with no tenant fails, instead of seeing everything", func(t *testing.T) {
		if _, err := db.Accounts.All(context.Background()); !errors.Is(err, models.ErrNoTenant) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("the filter reaches joins and subqueries by itself", func(t *testing.T) {
		journal := services.NewJournal(db)
		account(t, acme, db, "x", models.NoFloor)

		if _, err := journal.Post(
			acme,
			services.Transfer("x", "shared-code", 5, "mine", ""),
		); err != nil {
			t.Fatal(err)
		}

		visible, err := db.Accounts.Where(ant.Has(&models.Accounts.Lines)).All(theirs)
		if err != nil || len(visible) != 0 {
			t.Fatalf("%d, %v", len(visible), err)
		}
	})
}

func TestPreviewChangesNothing(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	account(t, ctx, db, "a", models.NoFloor)
	account(t, ctx, db, "b", models.NoFloor)

	preview, err := journal.Preview(ctx, services.Transfer("a", "b", 500, "maybe", ""))
	if err != nil {
		t.Fatal(err)
	}

	if preview.Balances[1].Amount != 500 || preview.Statements == 0 {
		t.Fatalf("preview: %+v", preview)
	}

	if got := balance(t, ctx, db, "b"); got != 0 {
		t.Fatalf("a preview moved money: %s", got)
	}
}

func TestReverse(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	account(t, ctx, db, "a", models.NoFloor)
	account(t, ctx, db, "b", models.NoFloor)

	entry, err := journal.Post(ctx, services.Transfer("a", "b", 500, "oops", ""))
	if err != nil {
		t.Fatal(err)
	}

	reversal, err := journal.Reverse(ctx, entry.Number, "undo")
	if err != nil {
		t.Fatal(err)
	}

	if balance(t, ctx, db, "b") != 0 || reversal.ReversalOf == nil {
		t.Fatalf("reversal: %+v", reversal)
	}

	if _, err := journal.Reverse(ctx, entry.Number, "again"); !errors.Is(err, services.ErrInvalid) {
		t.Fatalf("reversing twice: %v", err)
	}
}

func TestReports(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	account(t, ctx, db, "a", models.NoFloor)
	account(t, ctx, db, "b", models.NoFloor)

	for i := 1; i <= 3; i++ {
		if _, err := journal.Post(
			ctx,
			services.Transfer("a", "b", models.Money(i*100), "x", ""),
		); err != nil {
			t.Fatal(err)
		}
	}

	page, err := services.NewReports(db).Statement(ctx, "b", 1, 2)
	if err != nil {
		t.Fatal(err)
	}

	if page.Total != 3 || len(page.Items) != 2 || page.Items[0].Running != 600 ||
		page.Items[1].Running != 300 {
		t.Fatalf("statement: %+v", page.Items)
	}
}

func TestOutboxWorkersShareTheQueue(t *testing.T) {
	db, ctx := open(t)
	journal := services.NewJournal(db)
	account(t, ctx, db, "a", models.NoFloor)
	account(t, ctx, db, "b", models.NoFloor)

	for range 40 {
		if _, err := journal.Post(ctx, services.Transfer("a", "b", 1, "x", "")); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex

	seen := map[int64]int{}
	publish := func(m *models.Message) error {
		mu.Lock()
		defer mu.Unlock()

		seen[m.ID]++

		return nil
	}

	var wg sync.WaitGroup

	for range 4 {
		wg.Go(func() {
			for {
				sent, err := services.Send(context.Background(), db, 5, publish)
				if err != nil {
					t.Error(err)

					return
				}

				if sent == 0 {
					return
				}
			}
		})
	}

	wg.Wait()

	if len(seen) != 40 {
		t.Fatalf("%d of 40 sent", len(seen))
	}

	for id, times := range seen {
		if times != 1 {
			t.Fatalf("message %d sent %d times", id, times)
		}
	}

	t.Run("a failed publish returns the batch to the queue", func(t *testing.T) {
		if _, err := journal.Post(ctx, services.Transfer("a", "b", 1, "y", "")); err != nil {
			t.Fatal(err)
		}

		boom := errors.New("broker down")
		if _, err := services.Send(
			ctx,
			db,
			5,
			func(*models.Message) error { return boom },
		); !errors.Is(
			err,
			boom,
		) {
			t.Fatalf("got %v", err)
		}

		sent, err := services.Send(ctx, db, 5, publish)
		if err != nil || sent != 1 {
			t.Fatalf("sent %d, %v", sent, err)
		}
	})
}

func TestIBANsAreEncrypted(t *testing.T) {
	db, ctx := open(t)

	created, err := services.NewAccounts(db).Create(ctx, services.AccountInput{
		Code:     "bank",
		Name:     "Bank",
		Kind:     models.KindAsset,
		Currency: "EUR",
		IBAN:     "LV80BANK0000435195001",
	})
	if err != nil {
		t.Fatal(err)
	}

	stored, err := ant.Raw[string](
		db,
		"SELECT iban FROM accounts WHERE id = ?",
		created.ID,
	).First(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains([]byte(*stored), []byte("LV80")) {
		t.Fatal("the IBAN is in plain text")
	}

	loaded, err := db.Accounts.Find(ctx, created.ID)
	if err != nil || loaded.IBAN.V != "LV80BANK0000435195001" {
		t.Fatalf("loaded: %v, %v", loaded.IBAN.V, err)
	}
}

func TestStaleEditIsRefused(t *testing.T) {
	db, ctx := open(t)
	accounts := services.NewAccounts(db)

	a := account(t, ctx, db, "a", models.NoFloor)
	if _, err := accounts.Rename(ctx, "a", "Fresh", a.Version); err != nil {
		t.Fatal(err)
	}

	if _, err := accounts.Rename(
		ctx,
		"a",
		"Stale",
		a.Version,
	); !errors.Is(
		err,
		services.ErrConflict,
	) {
		t.Fatalf("got %v", err)
	}
}

func TestImportChart(t *testing.T) {
	db, ctx := open(t)
	accounts := services.NewAccounts(db)
	account(t, ctx, db, "income", models.NoFloor)

	created, updated, err := accounts.Import(ctx, []services.ImportNode{
		{
			Code:     "liabilities",
			Name:     "Liabilities",
			Kind:     models.KindLiability,
			Currency: "EUR",
			Children: []services.ImportNode{
				{
					Code:     "liabilities:loans",
					Name:     "Loans",
					Kind:     models.KindLiability,
					Currency: "EUR",
				},
			},
		},
		{
			Code:     "income",
			Name:     "Renamed",
			Kind:     models.KindAsset,
			Currency: "EUR",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if created != 2 || updated != 1 {
		t.Fatalf("created %d, updated %d", created, updated)
	}

	loans, err := db.Accounts.Where(ant.Eq(&models.Accounts.Code, "liabilities:loans")).
		IncludeOne(&models.Accounts.Parent).
		First(ctx)
	if err != nil || loans.Parent == nil || loans.Parent.Code != "liabilities" {
		t.Fatalf("loans: %+v, %v", loans, err)
	}
}

func TestStatementImportIsRepeatable(t *testing.T) {
	db, ctx := open(t)
	account(t, ctx, db, "bank", models.NoFloor)
	reconcile := services.NewReconcile(db)
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	lines := []services.BankLine{{"r1", 100, day}, {"r2", 200, day}}

	for range 2 {
		if _, err := reconcile.Import(ctx, "bank", lines); err != nil {
			t.Fatal(err)
		}
	}

	result, err := reconcile.Import(ctx, "bank", []services.BankLine{{"r2", 250, day}})
	if err != nil {
		t.Fatal(err)
	}

	if result.BankTotal != 350 {
		t.Fatalf("result: %+v", result)
	}

	if count, _ := db.Statement.Count(ctx); count != 2 {
		t.Fatalf("%d lines", count)
	}
}
