package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"lead-service/internal/bitrix"
	"lead-service/internal/handler"
	"lead-service/internal/repository"
	"lead-service/internal/service"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	bitrixURL := os.Getenv("BITRIX_BASE_URL")
	if bitrixURL == "" {
		log.Fatal("BITRIX_BASE_URL is required")
	}
	bitrixTimeout := 5 * time.Second
	if raw := os.Getenv("BITRIX_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatal(err)
		}
		bitrixTimeout = parsed
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	customers := service.NewCustomerService(repository.NewCustomerRepository(db))
	leads := service.NewLeadService(repository.NewLeadRepository(db), bitrix.New(bitrixURL, bitrixTimeout))
	imports := service.NewImportService(repository.NewImportRepository(db))
	qualification := service.NewQualificationService(repository.NewQualificationRepository(db))
	exports := service.NewExportService(repository.NewExportRepository(db), os.Getenv("SHEETS_BASE_URL"), os.Getenv("WEBHOOK_TIMEOUT"))

	r := chi.NewRouter()
	handler.New(customers, leads, imports, qualification, exports).Register(r)

	log.Fatal(http.ListenAndServe(addr, r))
}
