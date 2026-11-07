package worker

import (
	"context"
	"log/slog"
	"time"

	"lead-service/internal/repository"
)

// Reaper returns jobs stuck in_progress back to the queue.
type Reaper struct {
	repo  *repository.JobRepository
	every time.Duration
	after time.Duration
	log   *slog.Logger
}

func NewReaper(repo *repository.JobRepository, every, after time.Duration, logger *slog.Logger) *Reaper {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reaper{repo: repo, every: every, after: after, log: logger}
}

// Run performs a reaper pass every r.every until ctx is cancelled.
func (r *Reaper) Run(ctx context.Context) {
	ticker := time.NewTicker(r.every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.ReapOnce(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				r.log.Error("reaper", "err", err)
			}
		}
	}
}

// ReapOnce performs exactly one reaper pass. Tests call it directly instead
// of waiting for REAPER_EVERY.
func (r *Reaper) ReapOnce(ctx context.Context) (int64, error) {
	count, err := r.repo.Reap(ctx, r.after)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		r.log.Info("reaper requeued stale jobs", "count", count)
	}
	return count, nil
}
