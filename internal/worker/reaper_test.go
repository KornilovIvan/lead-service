package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"lead-service/internal/repository"
)

func TestReaperRequeuesStaleJob(t *testing.T) {
	db := openWorkerTestDB(t)
	ctx := context.Background()

	stale, leadID := insertClaimedJob(t, db)
	if _, err := db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'in_progress', locked_at = now() - interval '2 minutes'
		WHERE id = $1`, stale.ID); err != nil {
		t.Fatal(err)
	}

	fresh, _ := insertClaimedJob(t, db)

	reaper := NewReaper(repository.NewJobRepository(db), time.Minute, time.Minute, discardLogger())
	count, err := reaper.ReapOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("reaped %d jobs, want at least 1", count)
	}

	var status string
	var runAtAge float64
	var lockedAtNotNull bool
	if err := db.QueryRowContext(ctx, `
		SELECT status, EXTRACT(EPOCH FROM (now() - run_at))::float8, locked_at IS NOT NULL
		FROM jobs WHERE id = $1`, stale.ID).Scan(&status, &runAtAge, &lockedAtNotNull); err != nil {
		t.Fatal(err)
	}
	if status != "queued" {
		t.Fatalf("stale job status = %q, want queued", status)
	}
	if runAtAge < 0 || runAtAge >= 1 {
		t.Fatalf("run_at is %.3fs from now, want less than 1s", runAtAge)
	}
	if !lockedAtNotNull {
		t.Fatal("locked_at must be set by the reaper")
	}

	var bitrixID *string
	if err := db.GetContext(ctx, &bitrixID,
		`SELECT bitrix_id FROM leads WHERE id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if bitrixID != nil {
		t.Fatalf("bitrix_id = %q, want NULL", *bitrixID)
	}

	var freshStatus string
	if err := db.GetContext(ctx, &freshStatus,
		`SELECT status FROM jobs WHERE id = $1`, fresh.ID); err != nil {
		t.Fatal(err)
	}
	if freshStatus != "in_progress" {
		t.Fatalf("fresh job status = %q, want in_progress", freshStatus)
	}
}

func TestReaperInvalidatesOldLock(t *testing.T) {
	db := openWorkerTestDB(t)
	ctx := context.Background()
	job, leadID := insertClaimedJob(t, db)

	// Make the row stale; job.LockedAt still holds the lock from the "claim".
	if _, err := db.ExecContext(ctx, `
		UPDATE jobs SET locked_at = now() - interval '2 minutes' WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}

	reaper := NewReaper(repository.NewJobRepository(db), time.Minute, time.Minute, discardLogger())
	if _, err := reaper.ReapOnce(ctx); err != nil {
		t.Fatal(err)
	}

	repo := repository.NewJobRepository(db)
	if err := repo.Complete(ctx, job, "777"); !errors.Is(err, repository.ErrJobLockLost) {
		t.Fatalf("Complete err = %v, want ErrJobLockLost", err)
	}

	var status string
	if err := db.GetContext(ctx, &status,
		`SELECT status FROM jobs WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if status != "queued" {
		t.Fatalf("status = %q, want queued", status)
	}

	var bitrixID *string
	if err := db.GetContext(ctx, &bitrixID,
		`SELECT bitrix_id FROM leads WHERE id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if bitrixID != nil {
		t.Fatalf("bitrix_id = %q, want NULL after rollback", *bitrixID)
	}
}

func TestReaperSkipsRowLockedByAnotherTransaction(t *testing.T) {
	db := openWorkerTestDB(t)
	ctx := context.Background()

	first, _ := insertClaimedJob(t, db)
	second, _ := insertClaimedJob(t, db)
	for _, id := range []int{first.ID, second.ID} {
		if _, err := db.ExecContext(ctx, `
			UPDATE jobs SET locked_at = now() - interval '2 minutes' WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	var lockedID int
	if err := tx.GetContext(ctx, &lockedID,
		`SELECT id FROM jobs WHERE id = $1 FOR UPDATE`, first.ID); err != nil {
		t.Fatal(err)
	}

	reapCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	reaper := NewReaper(repository.NewJobRepository(db), time.Minute, time.Minute, discardLogger())
	count, err := reaper.ReapOnce(reapCtx)
	if err != nil {
		t.Fatalf("reaper blocked or failed: %v", err)
	}
	if count < 1 {
		t.Fatalf("reaped %d jobs, want at least 1", count)
	}

	var firstStatus string
	if err := db.GetContext(ctx, &firstStatus,
		`SELECT status FROM jobs WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "in_progress" {
		t.Fatalf("locked job status = %q, want in_progress", firstStatus)
	}

	var secondStatus string
	if err := db.GetContext(ctx, &secondStatus,
		`SELECT status FROM jobs WHERE id = $1`, second.ID); err != nil {
		t.Fatal(err)
	}
	if secondStatus != "queued" {
		t.Fatalf("unlocked job status = %q, want queued", secondStatus)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// After the lock is released the same row can be reaped too.
	if _, err := reaper.ReapOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.GetContext(ctx, &firstStatus,
		`SELECT status FROM jobs WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "queued" {
		t.Fatalf("after ROLLBACK status = %q, want queued", firstStatus)
	}
}
