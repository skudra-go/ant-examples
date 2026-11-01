package services

import (
	"context"
	"log/slog"

	"github.com/skudra-go/ant"
)

// AuditTrail logs what each SaveChanges is about to do, as whole entities and
// fields, not SQL. An error from SavingChanges would cancel the save, which is the
// place for a rule that spans everything in a unit of work.
type AuditTrail struct {
	log *slog.Logger
}

// NewAuditTrail returns the interceptor.
func NewAuditTrail(log *slog.Logger) *AuditTrail { return &AuditTrail{log} }

// Before implements ant.Interceptor.
func (a *AuditTrail) Before(ctx context.Context, _ *ant.Statement) (context.Context, error) {
	return ctx, nil
}

// After implements ant.Interceptor.
func (a *AuditTrail) After(context.Context, *ant.Statement, ant.Outcome) {}

// SavingChanges implements ant.SaveInterceptor.
func (a *AuditTrail) SavingChanges(ctx context.Context, changes []ant.Change) error {
	for _, change := range changes {
		a.log.Debug(
			"saving",
			"model",
			change.Model,
			"state",
			change.State.String(),
			"fields",
			change.Changed,
		)
	}

	return nil
}
