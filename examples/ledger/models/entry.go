package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skudra-go/ant"
)

// Entries is the field probe for Entry.
var Entries = ant.Fields[Entry]()

// Lines is the field probe for Line.
var Lines = ant.Fields[Line]()

// ErrUnbalanced reports an entry whose lines do not add up to zero.
var ErrUnbalanced = errors.New("the lines of an entry must add up to zero")

// Entry is one journal entry: money moving, recorded once and never changed. A
// mistake is corrected by a new entry that reverses it.
type Entry struct {
	Tenanted
	ID int64 `json:"id"`
	// Number comes from a database sequence, for a tidy human reference.
	Number int64 `json:"number"`
	// Key makes a request safe to repeat: a tenant cannot post one key twice.
	Key        *string   `json:"key,omitempty" ant:"immutable"`
	Memo       string    `json:"memo" ant:"immutable"`
	ReversalOf *int64    `json:"reversalOf,omitempty" ant:"immutable"`
	CreatedAt  time.Time `json:"createdAt"`
	Lines      []Line    `json:"lines"`
}

// Configure says what the struct cannot.
func (Entry) Configure(b *ant.ModelBuilder[Entry]) {
	b.Column(&Entries.Number).Sequence("entry_numbers", ant.StartAt(1000))

	b.Index(&Entries.TenantID).
		And(&Entries.Key).
		Unique().
		Where("key IS NOT NULL").
		Name("entries_tenant_key_key")

	b.HasMany(&Entries.Lines).ForeignKey(&Lines.EntryID).OnDelete(ant.Restrict)
}

// Validate implements ant.Validator. It runs before the entry is written, and the
// trigger in migrations/code checks the same at commit, in case anything bypasses Go.
func (e *Entry) Validate(context.Context) error {
	if len(e.Lines) < 2 {
		return fmt.Errorf("%w: an entry needs at least two lines", ErrUnbalanced)
	}

	var total Money

	for _, line := range e.Lines {
		total += line.Amount
	}

	if total != 0 {
		return fmt.Errorf("%w: off by %s", ErrUnbalanced, total)
	}

	return nil
}

// Line moves an amount in or out of one account: positive debits, negative credits.
type Line struct {
	Tenanted
	ID        int64     `json:"id"`
	EntryID   int64     `json:"entryId" ant:"immutable"`
	AccountID int64     `json:"accountId" ant:"immutable"`
	Amount    Money     `json:"amount" ant:"immutable"`
	CreatedAt time.Time `json:"createdAt"`

	// Running has no column. A statement fills it with a window function.
	Running Money `json:"running,omitempty" ant:"computed"`

	Entry   *Entry   `json:"-"`
	Account *Account `json:"-"`
}

// Configure says what the struct cannot.
func (Line) Configure(b *ant.ModelBuilder[Line]) {
	b.Check("lines_amount_not_zero", "amount <> 0")
	b.Index(&Lines.AccountID).And(&Lines.ID).Desc().Name("lines_account_id_idx")
	b.BelongsTo(&Lines.Account).ForeignKey(&Lines.AccountID).OnDelete(ant.Restrict)
	b.BelongsTo(&Lines.Entry).ForeignKey(&Lines.EntryID)
}
