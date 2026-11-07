package worker

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"lead-service/internal/bitrix"
	"lead-service/internal/metrics"
	"lead-service/internal/model"
	"lead-service/internal/repository"
)

// retryBackoff is how long a transiently failed job waits before the next try.
const retryBackoff = 30 * time.Second

// JobStore is the set of queue operations the worker needs.
type JobStore interface {
	Claim(ctx context.Context) (repository.Job, error)
	Complete(ctx context.Context, job repository.Job, bitrixID string) error
	Fail(ctx context.Context, job repository.Job) error
	Defer(ctx context.Context, job repository.Job, backoff time.Duration) error
}

// LeadStore loads the lead behind a claimed job.
type LeadStore interface {
	GetByID(ctx context.Context, id int) (model.Lead, error)
}

// RateLimiter reports whether one more Bitrix webhook call fits into the
// current window.
type RateLimiter interface {
	Allow(ctx context.Context) (bool, error)
}

type allowAll struct{}

func (allowAll) Allow(context.Context) (bool, error) { return true, nil }

type Worker struct {
	jobs    JobStore
	leads   LeadStore
	client  bitrix.Client
	limiter RateLimiter
	log     *slog.Logger
}

// Option customises a Worker.
type Option func(*Worker)

// WithLimiter installs a shared rate limiter. Without it the worker allows
// every call, which is only meant for tests and local wiring.
func WithLimiter(limiter RateLimiter) Option {
	return func(w *Worker) {
		if limiter != nil {
			w.limiter = limiter
		}
	}
}

func New(jobs JobStore, leads LeadStore, client bitrix.Client, logger *slog.Logger, opts ...Option) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	w := &Worker{jobs: jobs, leads: leads, client: client, limiter: allowAll{}, log: logger}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run claims jobs until ctx is cancelled. There is deliberately no poll
// interval: an idle worker tries to claim again immediately. WORKER_IDLE is
// logged once per idle period, when the worker transitions from busy to idle.
func (w *Worker) Run(ctx context.Context) {
	idleLogged := false
	for ctx.Err() == nil {
		processed, err := w.RunOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.Error("worker claim", "err", err)
			idleLogged = false
			continue
		}
		if !processed {
			if !idleLogged {
				w.log.Info("WORKER_IDLE")
				idleLogged = true
			}
			continue
		}
		idleLogged = false
	}
}

// RunOnce claims and processes at most one job. It returns false when the
// queue is empty.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, err := w.jobs.Claim(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	w.process(ctx, job)
	return true, nil
}

func (w *Worker) process(ctx context.Context, job repository.Job) {
	allowed, err := w.limiter.Allow(ctx)
	if err != nil {
		w.log.Error("worker rate limiter", "job_id", job.ID, "lead_id", job.LeadID, "err", err)
		w.deferJob(ctx, job)
		return
	}
	if !allowed {
		w.log.Warn("worker bitrix rate limited", "job_id", job.ID, "lead_id", job.LeadID)
		w.deferJob(ctx, job)
		return
	}

	lead, err := w.leads.GetByID(ctx, job.LeadID)
	if err != nil {
		w.log.Error("worker load lead", "job_id", job.ID, "lead_id", job.LeadID, "err", err)
		w.fail(ctx, job)
		return
	}

	bitrixID, err := bitrix.FindOrAdd(
		ctx,
		w.client,
		strconv.Itoa(lead.ID),
		lead.Name,
		stringValue(lead.Phone),
		stringValue(lead.Email),
	)
	if err != nil {
		w.handleBitrixError(ctx, job, lead.ID, err)
		return
	}

	if err := w.jobs.Complete(ctx, job, bitrixID); err != nil {
		w.log.Error("worker complete job", "job_id", job.ID, "lead_id", lead.ID, "err", err)
		return
	}
	w.log.Info("worker job done", "job_id", job.ID, "lead_id", lead.ID, "bitrix_id", bitrixID)
}

func (w *Worker) handleBitrixError(ctx context.Context, job repository.Job, leadID int, err error) {
	switch classify(err) {
	case actionRetry:
		w.log.Warn("worker bitrix retry", "job_id", job.ID, "lead_id", leadID, "err", err)
		w.deferJob(ctx, job)
	case actionWebhookRevoked:
		w.log.Error("webhook revoked", "job_id", job.ID, "lead_id", leadID, "err", err)
		w.fail(ctx, job)
	default:
		w.log.Error("worker bitrix failed", "job_id", job.ID, "lead_id", leadID, "err", err)
		w.fail(ctx, job)
	}
}

// deferJob puts the job back into the queue with a fixed 30s pause. The
// worker itself never sleeps, so the goroutine is free to take other jobs.
func (w *Worker) deferJob(ctx context.Context, job repository.Job) {
	if err := w.jobs.Defer(ctx, job, retryBackoff); err != nil {
		w.log.Error("worker defer job", "job_id", job.ID, "err", err)
	}
}

func (w *Worker) fail(ctx context.Context, job repository.Job) {
	if err := w.jobs.Fail(ctx, job); err != nil {
		w.log.Error("worker fail job", "job_id", job.ID, "err", err)
		return
	}
	metrics.SyncJobsFailedTotal.Inc()
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
