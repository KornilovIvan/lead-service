package worker

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/mock/gomock"

	"lead-service/internal/bitrix/mock"
	"lead-service/internal/model"
	"lead-service/internal/repository"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeJobs struct {
	job       repository.Job
	claimErr  error
	completed bool
	failed    bool
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
	f.completed = true
	f.bitrixID = bitrixID
	return nil
}

func (f *fakeJobs) Fail(context.Context, repository.Job) error {
	f.failed = true
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
