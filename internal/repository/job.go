package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// ErrJobLockLost is returned when a job is no longer owned by the worker that
// claimed it: the status changed or locked_at no longer matches.
var ErrJobLockLost = errors.New("job lock lost")

// Job is a claimed unit of work from the jobs table.
type Job struct {
	ID       int       `db:"id"`
	LeadID   int       `db:"lead_id"`
	LockedAt time.Time `db:"locked_at"`
}

type JobRepository struct {
	db *sqlx.DB
}

func NewJobRepository(db *sqlx.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Claim takes the oldest runnable job and commits the lock before returning,
// so the external HTTP call is made outside of any transaction.
//
// sql.ErrNoRows means the queue is empty right now.
func (r *JobRepository) Claim(ctx context.Context) (Job, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
		UPDATE jobs
		SET status = 'in_progress', locked_at = now()
		WHERE id = (
			SELECT id
			FROM jobs
			WHERE status = 'queued' AND run_at <= now()
			ORDER BY run_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, lead_id, locked_at`

	var job Job
	if err := tx.GetContext(ctx, &job, q); err != nil {
		return Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	return job, nil
}

// Complete writes the Bitrix id and marks the job done in one transaction.
// If the second UPDATE touches no rows, the transaction is rolled back and
// ErrJobLockLost is returned, so bitrix_id is never stored without the job.
func (r *JobRepository) Complete(ctx context.Context, job Job, bitrixID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE leads SET bitrix_id = $1 WHERE id = $2`, bitrixID, job.LeadID); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'done'
		WHERE id = $1 AND status = 'in_progress' AND locked_at = $2`, job.ID, job.LockedAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrJobLockLost
	}

	return tx.Commit()
}

// Fail marks a claimed job as failed so it can be retried from the API.
func (r *JobRepository) Fail(ctx context.Context, job Job) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'failed'
		WHERE id = $1 AND status = 'in_progress' AND locked_at = $2`, job.ID, job.LockedAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrJobLockLost
	}
	return nil
}

// Defer puts a claimed job back into the queue with a run_at in the future.
// It is a single guarded UPDATE: if the lock is no longer ours, nothing changes
// and ErrJobLockLost is returned.
func (r *JobRepository) Defer(ctx context.Context, job Job, backoff time.Duration) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'queued',
		    run_at = now() + make_interval(secs => $3),
		    locked_at = NULL
		WHERE id = $1 AND status = 'in_progress' AND locked_at = $2`,
		job.ID, job.LockedAt, int(backoff/time.Second))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrJobLockLost
	}
	return nil
}

// Reap returns stale in_progress jobs to the queue. run_at is set to now():
// the job is already overdue, so it must be claimable immediately, unlike a
// rate-limit deferral which adds a backoff.
func (r *JobRepository) Reap(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'queued', run_at = now(), locked_at = now()
		WHERE id IN (
			SELECT id
			FROM jobs
			WHERE status = 'in_progress'
			  AND locked_at < now() - $1::interval
			FOR UPDATE SKIP LOCKED
		)`, fmt.Sprintf("%d seconds", int64(olderThan/time.Second)))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
