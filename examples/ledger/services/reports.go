package services

import (
	"context"
	"time"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Reports reads the books.
type Reports struct {
	db *models.AppDB
}

// NewReports returns the service.
func NewReports(db *models.AppDB) *Reports { return &Reports{db} }

// Statement returns a page of an account's lines, newest first, each with the
// running balance after it. The window function runs in the database over every line
// of the account, before paging cuts them down, so page 50 shows true balances.
func (r *Reports) Statement(
	ctx context.Context,
	code string,
	page, size int,
) (*ant.Page[models.Line], error) {
	account, err := r.db.Accounts.Where(ant.Eq(&models.Accounts.Code, code)).
		Column(&models.Accounts.ID).
		First(ctx)
	if err != nil {
		return nil, translate(err)
	}

	return r.db.Lines.Where(ant.Eq(&models.Lines.AccountID, account.ID)).
		With(
			&models.Lines.Running,
			ant.Sum(&models.Lines.Amount).Over(ant.OrderedBy(ant.Col(&models.Lines.ID).Asc())),
		).
		IncludeOne(&models.Lines.Entry).
		Order(ant.Col(&models.Lines.ID).Desc()).
		Page(ctx, page, size)
}

// TrialLine is one row of a trial balance.
type TrialLine struct {
	Code    string       `json:"code"`
	Name    string       `json:"name"`
	Kind    string       `json:"kind"`
	Debits  models.Money `json:"debits"`
	Credits models.Money `json:"credits"`
	Net     models.Money `json:"net"`
}

// TrialLines is the field probe for TrialLine.
var TrialLines = ant.Fields[TrialLine]()

// TrialBalance adds up every line per account, as of a date. Debits and credits come
// from one scan, with conditional sums. By the rules of double entry the nets add up
// to zero, and a report that says so is the ledger proving itself.
type TrialBalance struct {
	AsOf     time.Time    `json:"asOf"`
	Lines    []TrialLine  `json:"lines"`
	Total    models.Money `json:"total"`
	Balanced bool         `json:"balanced"`
}

// Trial returns the trial balance.
func (r *Reports) Trial(ctx context.Context, asOf time.Time) (*TrialBalance, error) {
	// FILTER gives NULL for an account with no debits, which COALESCE turns to zero.
	debits := ant.Sum(&models.Lines.Amount).Filter(ant.Gt(&models.Lines.Amount, 0))
	credits := ant.Sum(&models.Lines.Amount).Filter(ant.Lt(&models.Lines.Amount, 0))

	lines, err := ant.Project[TrialLine](
		ant.Then(
			ant.Then(
				r.db.Lines.Where(ant.Lte(&models.Lines.CreatedAt, asOf)).
					Join(&models.Lines.Account).
					GroupBy(&models.Accounts.Code),
				&models.Accounts.Name,
			),
			&models.Accounts.Kind,
		).
			Order(ant.Col(&models.Accounts.Code).Asc()),
		ant.Into(&TrialLines.Code, &models.Accounts.Code),
		ant.Into(&TrialLines.Name, &models.Accounts.Name),
		ant.Into(&TrialLines.Kind, &models.Accounts.Kind),
		ant.IntoExpr(&TrialLines.Debits, ant.SQL[models.Money]("COALESCE(?, 0)::bigint", debits)),
		ant.IntoExpr(
			&TrialLines.Credits,
			ant.SQL[models.Money]("(-COALESCE(?, 0))::bigint", credits),
		),
		ant.IntoExpr(
			&TrialLines.Net,
			ant.SQL[models.Money]("SUM(?)::bigint", ant.Col(&models.Lines.Amount)),
		),
	).All(ctx)
	if err != nil {
		return nil, err
	}

	out := &TrialBalance{AsOf: asOf, Lines: lines}

	for _, line := range lines {
		out.Total += line.Net
	}

	out.Balanced = out.Total == 0

	return out, nil
}

// Activity is an account with its newest line.
type Activity struct {
	models.Account
	Last *models.Line `json:"lastLine,omitempty"`
}

// Recent returns accounts with their newest line. Latest on the relation makes
// IncludeOne read one row per parent, with a window function, however many lines
// each account has.
func (r *Reports) Recent(ctx context.Context) ([]Activity, error) {
	accounts, err := r.db.Accounts.IncludeOne(&models.Accounts.LastLine).
		OrderBy(&models.Accounts.Code).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Activity, len(accounts))

	for i, account := range accounts {
		out[i] = Activity{Account: *account, Last: account.LastLine}
	}

	return out, nil
}
