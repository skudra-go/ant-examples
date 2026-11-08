package services

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Journal posts entries, which is the heart of the ledger.
type Journal struct {
	db *models.AppDB
}

// NewJournal returns the service.
func NewJournal(db *models.AppDB) *Journal { return &Journal{db} }

// LineInput is one side of an entry.
type LineInput struct {
	Account string
	Amount  models.Money
}

// EntryInput is an entry to post. Key makes it safe to repeat.
type EntryInput struct {
	Key   string
	Memo  string
	Lines []LineInput
}

// Transfer moves money between two accounts: an entry with two lines.
func Transfer(from, to string, amount models.Money, memo, key string) EntryInput {
	return EntryInput{Key: key, Memo: memo, Lines: []LineInput{{from, -amount}, {to, amount}}}
}

// Post writes an entry, and is the one place balances change.
//
// Everything is one unit of work. The accounts are locked in key order, so two
// entries touching the same accounts wait for each other, and cannot deadlock. Then
// the code changes plain structs, the entry, its lines, each balance, and an outbox
// message, and SaveChanges works out the statements, in order, in one transaction.
//
// A session keeps this tracker apart from every other request. Locks are held to the
// end, so a balance read here is still true when it is written. If two requests do
// collide, the attempt runs again, which is safe because nothing outside the database
// has happened yet.
func (s *Journal) Post(ctx context.Context, input EntryInput) (*models.Entry, error) {
	session := ant.Session(s.db, ant.WithTracking(true))

	var entry *models.Entry

	err := session.Tx(
		ctx,
		func(tx *models.AppDB) error {
			var err error

			entry, err = s.post(ctx, tx, input)

			return err
		},
		ant.Attempts(5),
	)

	return entry, describe(err)
}

// Preview runs Post for real and rolls it back, so a caller sees exactly what would
// happen, including every rule the database enforces, and every statement.
func (s *Journal) Preview(ctx context.Context, input EntryInput) (*Preview, error) {
	session := ant.Session(s.db, ant.WithTracking(true))
	preview := &Preview{}

	statements, err := ant.DryRun(ctx, session, func(tx *models.AppDB) error {
		entry, err := s.post(ctx, tx, input)
		if err != nil {
			return err
		}

		preview.Entry = entry

		for _, line := range entry.Lines {
			account, err := tx.Accounts.Where(ant.Eq(&models.Accounts.ID, line.AccountID)).
				First(ctx)
			if err != nil {
				return err
			}

			preview.Balances = append(preview.Balances, Balance{account.Code, account.Balance})
		}

		return nil
	})
	if err != nil {
		return nil, describe(err)
	}

	preview.Statements = len(statements)

	return preview, nil
}

// Preview is what an entry would do.
type Preview struct {
	Entry      *models.Entry `json:"entry"`
	Balances   []Balance     `json:"balances"`
	Statements int           `json:"statements"`
}

// Balance is an account's balance.
type Balance struct {
	Account string       `json:"account"`
	Amount  models.Money `json:"amount"`
}

func (s *Journal) post(
	ctx context.Context,
	tx *models.AppDB,
	input EntryInput,
) (*models.Entry, error) {
	if input.Key != "" {
		// A repeat of a request that succeeded returns the first result.
		existing, err := tx.Entries.Where(ant.Eq(&models.Entries.Key, &input.Key)).
			NoTracking().Include(&models.Entries.Lines).First(ctx)
		if err == nil {
			return existing, nil
		}

		if !errors.Is(err, ant.ErrNotFound) {
			return nil, err
		}
	}

	if len(input.Lines) == 0 {
		return nil, fmt.Errorf("%w: an entry needs lines", ErrInvalid)
	}

	codes := make([]string, len(input.Lines))

	for i, line := range input.Lines {
		codes[i] = line.Account
	}

	accounts, err := tx.Accounts.Where(ant.In(&models.Accounts.Code, codes...)).
		OrderBy(&models.Accounts.ID).ForUpdate().All(ctx)
	if err != nil {
		return nil, err
	}

	byCode := make(map[string]*models.Account, len(accounts))

	for _, account := range accounts {
		byCode[account.Code] = account
	}

	entry := &models.Entry{Memo: input.Memo}

	if input.Key != "" {
		entry.Key = &input.Key
	}

	currency := ""

	for _, line := range input.Lines {
		account := byCode[line.Account]
		if account == nil {
			return nil, fmt.Errorf("%w: no account %s", ErrInvalid, line.Account)
		}

		if currency != "" && account.Currency != currency {
			return nil, fmt.Errorf(
				"%w: an entry cannot mix %s and %s",
				ErrInvalid,
				currency,
				account.Currency,
			)
		}

		currency = account.Currency
		account.Balance += line.Amount
		entry.Lines = append(entry.Lines, models.Line{AccountID: account.ID, Amount: line.Amount})
	}

	if err := ant.Add(ctx, tx, entry); err != nil {
		return nil, err
	}

	tenant, err := models.TenantOf(ctx)
	if err != nil {
		return nil, err
	}

	message := &models.Message{
		TenantID: tenant,
		Topic:    "entry.posted",
		Payload: ant.JSON[map[string]any]{
			V: map[string]any{"memo": input.Memo, "lines": len(input.Lines)},
		},
	}

	if err := ant.Add(ctx, tx, message); err != nil {
		return nil, err
	}

	if err := tx.SaveChanges(ctx); err != nil {
		return nil, err
	}

	return entry, nil
}

// describe turns database failures into what they mean here. The floor check and
// the balance trigger are both check violations, told apart by the constraint: the
// floor is the only one a correct entry can hit.
func describe(err error) error {
	var invalid *ant.ValidationError

	switch {
	case err == nil:
		return nil
	case errors.Is(err, models.ErrUnbalanced):
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	case errors.As(err, &invalid):
		return fmt.Errorf("%w: %s %v", ErrInvalid, invalid.Field, invalid.Err)
	case ant.IsCheckViolation(err):
		return ErrInsufficientFunds
	}

	return translate(err)
}

// Reverse posts an entry that undoes another. Nothing is edited, so history shows
// both what happened and that it was undone. Reversing twice is refused.
func (s *Journal) Reverse(ctx context.Context, number int64, memo string) (*models.Entry, error) {
	original, err := s.db.Entries.Where(ant.Eq(&models.Entries.Number, number)).
		Include(&models.Entries.Lines, func(q *ant.Query[models.Line]) *ant.Query[models.Line] {
			return q.IncludeOne(&models.Lines.Account)
		}).First(ctx)
	if err != nil {
		return nil, translate(err)
	}

	undone, err := s.db.Entries.Where(ant.Eq(&models.Entries.ReversalOf, &original.ID)).Exists(ctx)
	if err != nil {
		return nil, err
	}

	if undone {
		return nil, fmt.Errorf("%w: entry %d is already reversed", ErrInvalid, number)
	}

	input := EntryInput{Memo: memo, Key: fmt.Sprintf("reversal-%d", original.ID)}

	for _, line := range original.Lines {
		input.Lines = append(input.Lines, LineInput{line.Account.Code, -line.Amount})
	}

	slices.SortFunc(
		input.Lines,
		func(a, b LineInput) int { return cmp.Compare(a.Account, b.Account) },
	)

	session := ant.Session(s.db, ant.WithTracking(true))

	var entry *models.Entry

	err = session.Tx(
		ctx,
		func(tx *models.AppDB) error {
			var err error
			if entry, err = s.post(ctx, tx, input); err != nil {
				return err
			}

			_, err = ant.Raw[any](
				tx,
				"UPDATE entries SET reversal_of = ? WHERE id = ?",
				original.ID,
				entry.ID,
			).Exec(ctx)

			return err
		},
		ant.Attempts(5),
	)
	if err != nil {
		return nil, describe(err)
	}

	entry.ReversalOf = &original.ID

	return entry, nil
}
