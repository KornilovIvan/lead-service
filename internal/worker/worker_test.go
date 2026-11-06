package worker

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/mock/gomock"

	"lead-service/internal/bitrix"
	"lead-service/internal/bitrix/mock"
	"lead-service/internal/metrics"
	"lead-service/internal/model"
	"lead-service/internal/repository"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeJobs struct {
	job         repository.Job
	claimErr    error
	completeErr error
	failErr     error
	deferErr    error

	completed bool
	failed    bool
	deferred  bool
	backoff   time.Duration
	bitrixID  string
}

func (f *fakeJobs) Claim(context.Context) (repository.Job, error) {
	if f.claimErr != nil {
		return repository.Job{}, f.claimErr
	}
	if f.job.ID == 0 {
		return repository.Job{}, sql.ErrNoRows
	}
	return f.job, nil
}

func (f *fakeJobs) Complete(_ context.Context, _ repository.Job, bitrixID string) error {
	if f.completeErr != nil {
		return f.completeErr
	}
	f.completed = true
	f.bitrixID = bitrixID
	return nil
}

func (f *fakeJobs) Fail(context.Context, repository.Job) error {
	if f.failErr != nil {
		return f.failErr
	}
	f.failed = true
	return nil
}

func (f *fakeJobs) Defer(_ context.Context, _ repository.Job, backoff time.Duration) error {
	if f.deferErr != nil {
		return f.deferErr
	}
	f.deferred = true
	f.backoff = backoff
	return nil
}

type fakeLeads struct {
	lead model.Lead
	err  error
}

func (f *fakeLeads) GetByID(context.Context, int) (model.Lead, error) {
	return f.lead, f.err
}

func phone(v string) *string { return &v }

func TestRunOnceIdle(t *testing.T) {
	jobs := &fakeJobs{}
	w := New(jobs, &fakeLeads{}, nil, discardLogger())

	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if processed {
		t.Fatal("expected no job to be processed")
	}
}

func TestRunOnceAddsWhenNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)

	jobs := &fakeJobs{job: repository.Job{ID: 10, LeadID: 7}}
	leads := &fakeLeads{lead: model.Lead{ID: 7, Name: "Иван", Phone: phone("79001234567")}}

	client.EXPECT().FindByOriginID(gomock.Any(), "7").Return("", false, nil)
	client.EXPECT().AddLead(gomock.Any(), "7", "Иван", "79001234567", "").Return("555", nil)

	w := New(jobs, leads, client, discardLogger())
	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected job to be processed")
	}
	if jobs.failed {
		t.Fatal("job must not be failed")
	}
	if !jobs.completed || jobs.bitrixID != "555" {
		t.Fatalf("completed=%v bitrix_id=%q", jobs.completed, jobs.bitrixID)
	}
}

func TestRunOnceUsesExistingBitrixID(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)

	jobs := &fakeJobs{job: repository.Job{ID: 10, LeadID: 7}}
	leads := &fakeLeads{lead: model.Lead{ID: 7, Name: "Иван", Phone: phone("79001234567")}}

	// No AddLead expectation: a found lead must never be re-added.
	client.EXPECT().FindByOriginID(gomock.Any(), "7").Return("42", true, nil)

	w := New(jobs, leads, client, discardLogger())
	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected job to be processed")
	}
	if !jobs.completed || jobs.bitrixID != "42" {
		t.Fatalf("completed=%v bitrix_id=%q", jobs.completed, jobs.bitrixID)
	}
}

func TestRunOnceMarksFailedOnBitrixError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)

	jobs := &fakeJobs{job: repository.Job{ID: 10, LeadID: 7}}
	leads := &fakeLeads{lead: model.Lead{ID: 7, Name: "Иван", Phone: phone("79001234567")}}

	client.EXPECT().FindByOriginID(gomock.Any(), "7").Return("", false, errors.New("bitrix down"))

	w := New(jobs, leads, client, discardLogger())
	processed, err := w.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected job to be processed")
	}
	if !jobs.failed {
		t.Fatal("expected job to be marked failed")
	}
	if jobs.completed {
		t.Fatal("job must not be completed")
	}
}

func TestRunOnceClaimsError(t *testing.T) {
	jobs := &fakeJobs{claimErr: errors.New("db down")}
	w := New(jobs, &fakeLeads{}, nil, discardLogger())

	processed, err := w.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected claim error")
	}
	if processed {
		t.Fatal("expected no job to be processed")
	}
}

func TestClassifyErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantFailed   bool
		wantDeferred bool
		wantRevoked  bool
	}{
		{"408", &bitrix.HTTPError{Status: http.StatusRequestTimeout}, false, true, false},
		{"429", &bitrix.HTTPError{Status: http.StatusTooManyRequests}, false, true, false},
		{"400", &bitrix.HTTPError{Status: http.StatusBadRequest}, true, false, false},
		{"422", &bitrix.HTTPError{Status: http.StatusUnprocessableEntity}, true, false, false},
		{"401", &bitrix.HTTPError{Status: http.StatusUnauthorized}, true, false, true},
		{"timeout", context.DeadlineExceeded, false, true, false},
		{"network", &net.DNSError{Err: "no such host", Name: "bitrix"}, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := testutil.ToFloat64(metrics.SyncJobsFailedTotal)

			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)
			client.EXPECT().FindByOriginID(gomock.Any(), "7").Return("", false, tt.err)

			jobs := &fakeJobs{job: repository.Job{ID: 10, LeadID: 7}}
			leads := &fakeLeads{lead: model.Lead{ID: 7, Name: "Иван", Phone: phone("79001234567")}}
			var logs bytes.Buffer
			w := New(jobs, leads, client, slog.New(slog.NewTextHandler(&logs, nil)))

			if _, err := w.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}

			if jobs.failed != tt.wantFailed {
				t.Fatalf("failed = %v, want %v", jobs.failed, tt.wantFailed)
			}
			if jobs.deferred != tt.wantDeferred {
				t.Fatalf("deferred = %v, want %v", jobs.deferred, tt.wantDeferred)
			}
			if jobs.completed {
				t.Fatal("job must not be completed")
			}
			if tt.wantDeferred && jobs.backoff != retryBackoff {
				t.Fatalf("backoff = %v, want %v", jobs.backoff, retryBackoff)
			}
			if got := strings.Contains(logs.String(), "webhook revoked"); got != tt.wantRevoked {
				t.Fatalf("webhook revoked log = %v, want %v; logs: %s", got, tt.wantRevoked, logs.String())
			}

			wantDelta := 0.0
			if tt.wantFailed {
				wantDelta = 1
			}
			if got := testutil.ToFloat64(metrics.SyncJobsFailedTotal) - before; got != wantDelta {
				t.Fatalf("failed metric delta = %v, want %v", got, wantDelta)
			}
		})
	}
}

func TestFailedMetricNotIncrementedWhenLockLost(t *testing.T) {
	before := testutil.ToFloat64(metrics.SyncJobsFailedTotal)

	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)
	client.EXPECT().
		FindByOriginID(gomock.Any(), "7").
		Return("", false, &bitrix.HTTPError{Status: http.StatusUnprocessableEntity})

	jobs := &fakeJobs{
		job:     repository.Job{ID: 10, LeadID: 7},
		failErr: repository.ErrJobLockLost,
	}
	leads := &fakeLeads{lead: model.Lead{ID: 7, Name: "Иван", Phone: phone("79001234567")}}
	w := New(jobs, leads, client, discardLogger())

	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if jobs.failed {
		t.Fatal("fake must report a lost lock, not a failed transition")
	}
	if got := testutil.ToFloat64(metrics.SyncJobsFailedTotal) - before; got != 0 {
		t.Fatalf("failed metric delta = %v, want 0", got)
	}
}

type countingHandler struct {
	mu   sync.Mutex
	idle int
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *countingHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "WORKER_IDLE" {
		h.mu.Lock()
		h.idle++
		h.mu.Unlock()
	}
	return nil
}

func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *countingHandler) WithGroup(string) slog.Handler      { return h }

func (h *countingHandler) idleLogs() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.idle
}

type idlingJobs struct {
	calls  atomic.Int32
	cancel context.CancelFunc
}

func (f *idlingJobs) Claim(context.Context) (repository.Job, error) {
	if f.calls.Add(1) == 3 {
		f.cancel()
	}
	return repository.Job{}, sql.ErrNoRows
}

func (f *idlingJobs) Complete(context.Context, repository.Job, string) error { return nil }
func (f *idlingJobs) Fail(context.Context, repository.Job) error             { return nil }
func (f *idlingJobs) Defer(context.Context, repository.Job, time.Duration) error {
	return nil
}

func TestRunLogsIdleOncePerIdlePeriod(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := &countingHandler{}
	jobs := &idlingJobs{cancel: cancel}
	w := New(jobs, &fakeLeads{}, nil, slog.New(handler))

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	<-done

	if got := handler.idleLogs(); got != 1 {
		t.Fatalf("WORKER_IDLE logs = %d, want 1", got)
	}
}
