package worker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/mock/gomock"

	"lead-service/internal/bitrix"
	"lead-service/internal/bitrix/mock"
	"lead-service/internal/metrics"
	"lead-service/internal/repository"
	"lead-service/internal/testdb"
)

var workerTestPhone atomic.Int64

func openWorkerTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return testdb.Open(t)
}

// seededJobs skips Claim and returns a job that the test already inserted as
// claimed. Complete, Fail and Defer come from the real repository, so the
// tests exercise the SQL predicates against PostgreSQL.
type seededJobs struct {
	*repository.JobRepository
	job repository.Job
}

func (s seededJobs) Claim(context.Context) (repository.Job, error) {
	return s.job, nil
}

func insertClaimedJob(t *testing.T, db *sqlx.DB) (repository.Job, int) {
	t.Helper()
	ctx := context.Background()

	var customerID int
	if err := db.GetContext(ctx, &customerID,
		`INSERT INTO customers (name) VALUES ($1) RETURNING id`,
		fmt.Sprintf("worker-it-%d", time.Now().UnixNano())); err != nil {
		t.Fatal(err)
	}

	phone := fmt.Sprintf("78%09d", workerTestPhone.Add(1))
	var leadID int
	if err := db.GetContext(ctx, &leadID, `
		INSERT INTO leads (customer_id, name, phone, status)
		VALUES ($1, 'worker-it', $2, 'new')
		RETURNING id`, customerID, phone); err != nil {
		t.Fatal(err)
	}

	var job repository.Job
	if err := db.GetContext(ctx, &job, `
		INSERT INTO jobs (lead_id, status, run_at, locked_at)
		VALUES ($1, 'in_progress', now(), now())
		RETURNING id, lead_id, locked_at`, leadID); err != nil {
		t.Fatal(err)
	}
	return job, leadID
}

func TestTransientErrorsRequeueJob(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"429", http.StatusTooManyRequests},
		{"408", http.StatusRequestTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openWorkerTestDB(t)
			ctx := context.Background()
			job, leadID := insertClaimedJob(t, db)

			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)
			client.EXPECT().
				FindByOriginID(gomock.Any(), strconv.Itoa(leadID)).
				Return("", false, &bitrix.HTTPError{Status: tt.status})

			w := New(
				seededJobs{JobRepository: repository.NewJobRepository(db), job: job},
				repository.NewLeadRepository(db),
				client,
				discardLogger(),
			)
			if _, err := w.RunOnce(ctx); err != nil {
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
			if seconds < 20 {
				t.Fatalf("run_at in %.1fs, want >= 20s", seconds)
			}
		})
	}
}

func TestPermanentErrorsFailJob(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"400", http.StatusBadRequest},
		{"422", http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openWorkerTestDB(t)
			ctx := context.Background()
			job, leadID := insertClaimedJob(t, db)

			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)
			client.EXPECT().
				FindByOriginID(gomock.Any(), strconv.Itoa(leadID)).
				Return("", false, &bitrix.HTTPError{Status: tt.status})

			before := testutil.ToFloat64(metrics.SyncJobsFailedTotal)
			w := New(
				seededJobs{JobRepository: repository.NewJobRepository(db), job: job},
				repository.NewLeadRepository(db),
				client,
				discardLogger(),
			)
			if _, err := w.RunOnce(ctx); err != nil {
				t.Fatal(err)
			}

			var status string
			if err := db.GetContext(ctx, &status,
				`SELECT status FROM jobs WHERE id = $1`, job.ID); err != nil {
				t.Fatal(err)
			}
			if status != "failed" {
				t.Fatalf("status = %q, want failed", status)
			}

			after := testutil.ToFloat64(metrics.SyncJobsFailedTotal)
			if after <= before {
				t.Fatalf("failed counter did not grow: before=%v after=%v", before, after)
			}
			if after <= 0 {
				t.Fatalf("failed counter = %v, want > 0", after)
			}
		})
	}
}

func TestUnauthorizedFailsAndLogsWebhookRevoked(t *testing.T) {
	db := openWorkerTestDB(t)
	ctx := context.Background()
	job, leadID := insertClaimedJob(t, db)

	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)
	client.EXPECT().
		FindByOriginID(gomock.Any(), strconv.Itoa(leadID)).
		Return("", false, &bitrix.HTTPError{Status: http.StatusUnauthorized})

	var logs bytes.Buffer
	w := New(
		seededJobs{JobRepository: repository.NewJobRepository(db), job: job},
		repository.NewLeadRepository(db),
		client,
		slog.New(slog.NewTextHandler(&logs, nil)),
	)
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := db.GetContext(ctx, &status,
		`SELECT status FROM jobs WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	if !strings.Contains(logs.String(), "webhook revoked") {
		t.Fatalf("log does not contain webhook revoked: %s", logs.String())
	}
}

func TestWorkerDoesNotWriteResultWhenLockChanged(t *testing.T) {
	db := openWorkerTestDB(t)
	ctx := context.Background()
	job, leadID := insertClaimedJob(t, db)

	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)
	client.EXPECT().
		FindByOriginID(gomock.Any(), strconv.Itoa(leadID)).
		DoAndReturn(func(ctx context.Context, _ string) (string, bool, error) {
			// While the worker is in flight, someone else takes the lock.
			if _, err := db.ExecContext(ctx,
				`UPDATE jobs SET locked_at = locked_at + interval '1 second' WHERE id = $1`,
				job.ID); err != nil {
				t.Fatalf("change locked_at: %v", err)
			}
			return "999", true, nil
		})

	w := New(
		seededJobs{JobRepository: repository.NewJobRepository(db), job: job},
		repository.NewLeadRepository(db),
		client,
		discardLogger(),
	)
	if _, err := w.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := db.GetContext(ctx, &status,
		`SELECT status FROM jobs WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if status != "in_progress" {
		t.Fatalf("status = %q, want in_progress after zero-row UPDATE", status)
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
