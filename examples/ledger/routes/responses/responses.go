// Package responses holds the shapes the API returns.
package responses

import (
	"github.com/skudra-go/ant"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Statement is a page of an account's lines.
type Statement struct {
	Lines []Line `json:"lines"`
	Total int64  `json:"total"`
	Page  int    `json:"page"`
	Pages int    `json:"pages"`
}

// Line is a line with its entry's memo.
type Line struct {
	models.Line
	Number int64  `json:"entry"`
	Memo   string `json:"memo"`
}

// NewStatement shapes a page.
func NewStatement(page *ant.Page[models.Line]) Statement {
	out := Statement{
		Lines: make([]Line, len(page.Items)),
		Total: page.Total,
		Page:  page.Number,
		Pages: page.Pages(),
	}

	for i, line := range page.Items {
		out.Lines[i] = Line{Line: *line, Number: line.Entry.Number, Memo: line.Entry.Memo}
	}

	return out
}

// Subtree is an account and everything under it.
type Subtree struct {
	Accounts []*models.Account `json:"accounts"`
	Total    models.Money      `json:"total"`
}
