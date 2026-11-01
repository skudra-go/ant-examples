package models

import (
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/secure"
	"github.com/skudra-go/ant/validate"
)

// Accounts is the field probe for Account.
var Accounts = ant.Fields[Account]()

// Kind values. Assets and expenses grow with debits, and the rest with credits,
// but a line's sign is all this ledger needs: debits positive, credits negative.
const (
	KindAsset     = "asset"
	KindLiability = "liability"
	KindEquity    = "equity"
	KindIncome    = "income"
	KindExpense   = "expense"
)

// NoFloor is the Floor of an account that may go negative, as equity and income do.
const NoFloor Money = -9_000_000_000_000_000

// Account is a place money sits, in a tree: Assets, then Bank, then Bank:EUR.
type Account struct {
	Tenanted
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parentId,omitempty"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Currency string `json:"currency"`
	// Balance is the sum of the account's lines, kept up to date by every entry.
	Balance Money `json:"balance"`
	// Floor is the lowest the balance may go. A check constraint enforces it, so
	// no bug in Go can overdraw an account. It has no column default on purpose: a
	// default in ant means a zero field is left to the database, and zero is the most
	// common floor there is. NoFloor says an account may go as low as it likes.
	Floor Money `json:"floor"`
	// IBAN is encrypted in the database.
	IBAN secure.Value[string] `json:"-"`

	// Version counts writes, so that a stale edit is refused.
	Version   int64         `json:"version" ant:"version"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	ClosedAt  ant.DeletedAt `json:"-"`

	Parent   *Account  `json:"-"`
	Children []Account `json:"-"`
	Lines    []Line    `json:"-"`
	// LastLine is the newest of Lines, for showing activity without loading it all.
	LastLine *Line `json:"-"`
}

// Configure says what the struct cannot.
func (Account) Configure(b *ant.ModelBuilder[Account]) {
	b.Index(&Accounts.TenantID).And(&Accounts.Code).Unique().Name("accounts_tenant_code_key")

	b.Column(&Accounts.Kind).
		Enum("account_kind", KindAsset, KindLiability, KindEquity, KindIncome, KindExpense)

	b.Column(&Accounts.Currency).Fixed(3)
	b.Check("accounts_balance_floor", "balance >= floor")
	b.Check("accounts_currency_upper", "currency = upper(currency)")
	b.Column(&Accounts.IBAN).Sensitive()

	b.Validate(&Accounts.Code, validate.All(validate.Required[string](), validate.MaxLength(40)))

	b.Validate(
		&Accounts.Kind,
		validate.OneOf(KindAsset, KindLiability, KindEquity, KindIncome, KindExpense),
	)

	b.Validate(&Accounts.Currency, validate.MinLength(3))

	b.BelongsTo(&Accounts.Parent).ForeignKey(&Accounts.ParentID).OnDelete(ant.Restrict)
	b.HasMany(&Accounts.Children).ForeignKey(&Accounts.ParentID).Inverse(&Accounts.Parent)
	b.HasMany(&Accounts.Lines).ForeignKey(&Lines.AccountID)
	b.HasOne(&Accounts.LastLine).ForeignKey(&Lines.AccountID).Latest(&Lines.ID)
}
