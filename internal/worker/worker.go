package worker

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"

	"lead-service/internal/bitrix"
	"lead-service/internal/model"
	"lead-service/internal/repository"
)

// JobStore is the set of queue operations the worker needs.
type JobStore interface {
	Claim(ctx context.Context) (repository.Job, error)
	Complete(ctx context.Context, job repository.Job, bitrixID string) error
	Fail(ctx context.Context, job repository.Job) error
}

// LeadStore loads the lead behind a claimed job.
type LeadStore interface {
	GetByID(ctx context.Context, id int) (model.Lead, error)
}

type Worker struct {
	jobs   JobStore
	leads  LeadStore
	client bitrix.Client
	log    *slog.Logger
}

func New(jobs JobStore, leads LeadStore, client bitrix.Client, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{jobs: jobs, leads: leads, client: client, log: logger}
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
		w.log.Error("worker bitrix sync", "job_id", job.ID, "lead_id", lead.ID, "err", err)
		w.fail(ctx, job)
		return
	}

	if err := w.jobs.Complete(ctx, job, bitrixID); err != nil {
		w.log.Error("worker complete job", "job_id", job.ID, "lead_id", lead.ID, "err", err)
		return
	}
	w.log.Info("worker job done", "job_id", job.ID, "lead_id", lead.ID, "bitrix_id", bitrixID)
}

func (w *Worker) fail(ctx context.Context, job repository.Job) {
	if err := w.jobs.Fail(ctx, job); err != nil {
		w.log.Error("worker fail job", "job_id", job.ID, "err", err)
	}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
