package repository

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/testdb"
)

var jobTestPhone atomic.Int64

// claimedJob inserts a lead and a job that is already claimed by us, so the
// tests below never depend on other rows in the global jobs queue.
func claimedJob(t *testing.T, db *sqlx.DB) (Job, int) {
	t.Helper()
	ctx := context.Background()

	var customerID int
	if err := db.GetContext(ctx, &customerID,
		`INSERT INTO customers (name) VALUES ($1) RETURNING id`,
		fmt.Sprintf("job-test-%d", time.Now().UnixNano())); err != nil {
		t.Fatal(err)
	}

	phone := fmt.Sprintf("79%09d", jobTestPhone.Add(1))
	var leadID int
	if err := db.GetContext(ctx, &leadID, `
		INSERT INTO leads (customer_id, name, phone, status)
		VALUES ($1, 'job-test', $2, 'new')
		RETURNING id`, customerID, phone); err != nil {
		t.Fatal(err)
	}

	var job Job
	if err := db.GetContext(ctx, &job, `
		INSERT INTO jobs (lead_id, status, run_at, locked_at)
		VALUES ($1, 'in_progress', now(), now())
		RETURNING id, lead_id, locked_at`, leadID); err != nil {
		t.Fatal(err)
	}
	return job, leadID
}

func changeLock(t *testing.T, db *sqlx.DB, jobID int) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`UPDATE jobs SET locked_at = locked_at + interval '1 second' WHERE id = $1`,
		jobID); err != nil {
		t.Fatal(err)
	}
}

func jobStatus(t *testing.T, db *sqlx.DB, jobID int) string {
	t.Helper()
	var status string
	if err := db.GetContext(context.Background(), &status,
		`SELECT status FROM jobs WHERE id = $1`, jobID); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestCompleteRollsBackWhenLockChanged(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	repo := NewJobRepository(db)
	job, leadID := claimedJob(t, db)

	// Simulate a stale lock by hand: no reaper exists in this task.
	changeLock(t, db, job.ID)

	if err := repo.Complete(ctx, job, "777"); !errors.Is(err, ErrJobLockLost) {
		t.Fatalf("Complete err = %v, want ErrJobLockLost", err)
	}

	var bitrixID *string
	if err := db.GetContext(ctx, &bitrixID,
		`SELECT bitrix_id FROM leads WHERE id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if bitrixID != nil {
		t.Fatalf("bitrix_id = %q, want NULL after rollback", *bitrixID)
	}
	if status := jobStatus(t, db, job.ID); status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", status)
	}
}

func TestFailAndDeferRespectLock(t *testing.T) {
	tests := []struct {
		name string
		call func(context.Context, *JobRepository, Job) error
	}{
		{
			name: "fail",
			call: func(ctx context.Context, repo *JobRepository, job Job) error {
				return repo.Fail(ctx, job)
			},
		},
		{
			name: "defer",
			call: func(ctx context.Context, repo *JobRepository, job Job) error {
				return repo.Defer(ctx, job, 30*time.Second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.Open(t)
			ctx := context.Background()
			repo := NewJobRepository(db)
			job, _ := claimedJob(t, db)

			changeLock(t, db, job.ID)

			if err := tt.call(ctx, repo, job); !errors.Is(err, ErrJobLockLost) {
				t.Fatalf("err = %v, want ErrJobLockLost", err)
			}
			if status := jobStatus(t, db, job.ID); status != "in_progress" {
				t.Fatalf("status = %q после нулевого UPDATE, want in_progress", status)
			}
		})
	}
}

func TestCompleteWritesBitrixID(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	repo := NewJobRepository(db)
	job, leadID := claimedJob(t, db)

	if err := repo.Complete(ctx, job, "777"); err != nil {
		t.Fatal(err)
	}

	var bitrixID *string
	if err := db.GetContext(ctx, &bitrixID,
		`SELECT bitrix_id FROM leads WHERE id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if bitrixID == nil || *bitrixID != "777" {
		t.Fatalf("bitrix_id = %v, want 777", bitrixID)
	}
	if status := jobStatus(t, db, job.ID); status != "done" {
		t.Fatalf("status = %q, want done", status)
	}
}

func TestDeferRequeuesWithBackoff(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	repo := NewJobRepository(db)
	job, _ := claimedJob(t, db)

	if err := repo.Defer(ctx, job, 30*time.Second); err != nil {
		t.Fatal(err)
	}

	var status string
	var lockedAtIsNull bool
	var seconds float64
	if err := db.QueryRowContext(ctx, `
		SELECT status, locked_at IS NULL, EXTRACT(EPOCH FROM (run_at - now()))::float8
		FROM jobs WHERE id = $1`, job.ID).Scan(&status, &lockedAtIsNull, &seconds); err != nil {
		t.Fatal(err)
	}
	if status != "queued" {
		t.Fatalf("status = %q, want queued", status)
	}
	if !lockedAtIsNull {
		t.Fatal("locked_at must be NULL after defer")
	}
	if seconds < 29 || seconds > 31 {
		t.Fatalf("run_at in %.2fs, want ~30s", seconds)
	}
}

func TestFailMarksJobFailed(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	repo := NewJobRepository(db)
	job, _ := claimedJob(t, db)

	if err := repo.Fail(ctx, job); err != nil {
		t.Fatal(err)
	}
	if status := jobStatus(t, db, job.ID); status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
}
