package services

import (
	"context"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Reconcile compares the books with what the bank says.
type Reconcile struct {
	db *models.AppDB
}

// NewReconcile returns the service.
func NewReconcile(db *models.AppDB) *Reconcile { return &Reconcile{db} }

// BankLine is a line of a bank statement.
type BankLine struct {
	Reference string
	Amount    models.Money
	BookedOn  time.Time
}

// Result says how an upload compares with the books.
type Result struct {
	Stored     int64        `json:"stored"`
	BankTotal  models.Money `json:"bankTotal"`
	BookTotal  models.Money `json:"bookTotal"`
	Difference models.Money `json:"difference"`
}

// Import loads a statement, which banks send again and again, with corrections. One
// MERGE takes all the rows at once, held in memory with ant.Values: a line seen before
// updates in place if its amount changed, and a new line is inserted. The whole file
// is one statement, so a repeat changes nothing.
func (s *Reconcile) Import(ctx context.Context, code string, lines []BankLine) (*Result, error) {
	account, err := s.db.Accounts.Where(ant.Eq(&models.Accounts.Code, code)).First(ctx)
	if err != nil {
		return nil, translate(err)
	}

	tenant, err := models.TenantOf(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]models.StagedLine, len(lines))

	for i, line := range lines {
		rows[i] = models.StagedLine{
			TenantID:  tenant,
			Reference: line.Reference,
			AccountID: account.ID,
			Amount:    line.Amount,
			BookedOn:  line.BookedOn,
		}
	}

	statement, staged := models.StatementLines, models.StagedLines

	stored, err := ant.Merge(
		&s.db.Statement,
		ant.Values(s.db, rows...),
		ant.EqCol(&statement.TenantID, &staged.TenantID),
		ant.EqCol(&statement.Reference, &staged.Reference),
	).
		WhenMatched(
			ant.SetExpr(&statement.Amount, ant.Col(&staged.Amount)),
			ant.SetExpr(&statement.BookedOn, ant.Col(&staged.BookedOn)),
		).
		WhenNotMatched(
			ant.Into(&statement.TenantID, &staged.TenantID),
			ant.Into(&statement.Reference, &staged.Reference),
			ant.Into(&statement.AccountID, &staged.AccountID),
			ant.Into(&statement.Amount, &staged.Amount),
			ant.Into(&statement.BookedOn, &staged.BookedOn),
		).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	bank, err := ant.SumOf(
		ctx,
		s.db.Statement.Where(ant.Eq(&statement.AccountID, account.ID)),
		&statement.Amount,
	)
	if err != nil {
		return nil, err
	}

	return &Result{
		Stored:     stored,
		BankTotal:  bank,
		BookTotal:  account.Balance,
		Difference: account.Balance - bank,
	}, nil
}
