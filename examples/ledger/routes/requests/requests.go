// Package requests holds the shapes the API accepts.
package requests

import (
	"time"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Account is the body of POST /accounts.
type Account struct {
	Code     string        `json:"code"`
	Name     string        `json:"name"`
	Kind     string        `json:"kind"`
	Currency string        `json:"currency"`
	Parent   string        `json:"parent"`
	IBAN     string        `json:"iban"`
	Floor    *models.Money `json:"floor"`
}

// Rename is the body of PATCH /accounts/{code}. Version is what the caller read.
type Rename struct {
	Name    string `json:"name"`
	Version int64  `json:"version"`
}

// Transfer is the body of POST /transfers.
type Transfer struct {
	From   string       `json:"from"`
	To     string       `json:"to"`
	Amount models.Money `json:"amount"`
	Memo   string       `json:"memo"`
	Key    string       `json:"key"`
}

// Line is one side of an entry.
type Line struct {
	Account string       `json:"account"`
	Amount  models.Money `json:"amount"`
}

// Entry is the body of POST /entries.
type Entry struct {
	Memo  string `json:"memo"`
	Key   string `json:"key"`
	Lines []Line `json:"lines"`
}

// Reverse is the body of POST /entries/{number}/reverse.
type Reverse struct {
	Memo string `json:"memo"`
}

// ChartNode is an account in a chart sent as one tree, to PUT /chart.
type ChartNode struct {
	Code     string      `json:"code"`
	Name     string      `json:"name"`
	Kind     string      `json:"kind"`
	Currency string      `json:"currency"`
	Children []ChartNode `json:"children"`
}

// BankLine is one line of the body of POST /accounts/{code}/statement.
type BankLine struct {
	Reference string       `json:"reference"`
	Amount    models.Money `json:"amount"`
	BookedOn  time.Time    `json:"bookedOn"`
}
