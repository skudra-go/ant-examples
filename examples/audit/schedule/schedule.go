// Package schedule runs work on a cron schedule.
package schedule

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// Start runs job on a standard five-field cron spec, in UTC, until ctx ends. An
// empty spec runs nothing. A run that is still going when the next one is due is
// skipped, not stacked.
func Start(
	ctx context.Context,
	log *slog.Logger,
	spec string,
	job func(context.Context) error,
) error {
	if spec == "" {
		return nil
	}

	scheduler := cron.New(
		cron.WithLocation(time.UTC),
		cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)),
	)

	_, err := scheduler.AddFunc(spec, func() {
		if err := job(ctx); err != nil {
			log.Error("scheduled job failed", "error", err)
		}
	})
	if err != nil {
		return err
	}

	scheduler.Start()
	log.Info("schedule started", "cron", spec)

	go func() {
		<-ctx.Done()
		<-scheduler.Stop().Done()
	}()

	return nil
}
