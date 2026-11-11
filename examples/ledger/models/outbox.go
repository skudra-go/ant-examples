package models

import (
	"time"

	"github.com/skudra-go/ant"
)

// Messages is the probe for Message.
var Messages = ant.Fields[Message]()

// Message is something to tell the outside world. It is written in the same
// transaction as the change it describes, so a message exists exactly when the money
// moved, and a worker sends it afterwards. Sending from the request instead would
// lose messages in a crash, or announce changes that then rolled back.
type Message struct {
	ID          int64                    `json:"id"`
	TenantID    int64                    `json:"tenantId"`
	Topic       string                   `json:"topic"`
	Payload     ant.JSON[map[string]any] `json:"payload"`
	CreatedAt   time.Time                `json:"createdAt"`
	PublishedAt *time.Time               `json:"publishedAt,omitempty"`
}

// Configure says what the struct cannot.
func (Message) Configure(b *ant.ModelBuilder[Message]) {
	b.Table("outbox")
	b.Index(&Messages.ID).Where("published_at IS NULL").Name("outbox_pending_idx")
}

// StatementLines is the probe for StatementLine.
var StatementLines = ant.Fields[StatementLine]()

// StatementLine is one line of a bank statement, kept to reconcile against.
type StatementLine struct {
	Tenanted
	ID        int64     `json:"id"`
	Reference string    `json:"reference"`
	AccountID int64     `json:"accountId"`
	Amount    Money     `json:"amount"`
	BookedOn  time.Time `json:"bookedOn"`
	Account   *Account  `json:"-"`
}

// Configure says what the struct cannot.
func (StatementLine) Configure(b *ant.ModelBuilder[StatementLine]) {
	b.Table("statement_lines")
	b.Column(&StatementLines.BookedOn).Type("date")

	b.Index(&StatementLines.TenantID).
		And(&StatementLines.Reference).
		Unique().
		Name("statement_lines_tenant_reference_key")

	b.BelongsTo(&StatementLines.Account).ForeignKey(&StatementLines.AccountID)
}

// StagedLine is a statement line in memory, on its way in. It has no table: ant.Values
// reads rows of it as if it did.
type StagedLine struct {
	TenantID  int64
	Reference string
	AccountID int64
	Amount    Money
	BookedOn  time.Time
}

// Configure says what the struct cannot.
func (StagedLine) Configure(b *ant.ModelBuilder[StagedLine]) {
	b.ExcludeFromMigrations()
	b.Column(&StagedLines.BookedOn).Type("date")
}

// StagedLines is the probe for StagedLine.
var StagedLines = ant.Fields[StagedLine]()
