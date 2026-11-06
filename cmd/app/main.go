package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"lead-service/internal/bitrix"
	"lead-service/internal/handler"
	"lead-service/internal/repository"
	"lead-service/internal/service"
	"lead-service/internal/worker"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dsn := os.Getenv("DATABASE_URL")
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	leadRepo := repository.NewLeadRepository(db)
	jobRepo := repository.NewJobRepository(db)

	customers := service.NewCustomerService(repository.NewCustomerRepository(db))
	leads := service.NewLeadService(leadRepo)
	imports := service.NewImportService(repository.NewImportRepository(db))
	qualification := service.NewQualificationService(repository.NewQualificationRepository(db))
	exports := service.NewExportService(repository.NewExportRepository(db), os.Getenv("SHEETS_BASE_URL"), os.Getenv("WEBHOOK_TIMEOUT"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bitrixBaseURL := os.Getenv("BITRIX_BASE_URL")
	if bitrixBaseURL == "" {
		logger.Warn("BITRIX_BASE_URL is empty, bitrix sync jobs will fail")
	}
	bitrixClient := bitrix.New(bitrixBaseURL, durationEnv("BITRIX_TIMEOUT", 5*time.Second))

	w := worker.New(jobRepo, leadRepo, bitrixClient, logger)
	concurrency := intEnv("WORKER_CONCURRENCY", 4)
	for i := 0; i < concurrency; i++ {
		go w.Run(ctx)
	}
	logger.Info("worker started", "concurrency", concurrency)

	r := chi.NewRouter()
	handler.New(customers, leads, imports, qualification, exports).Register(r)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func intEnv(name string, def int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func durationEnv(name string, def time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
