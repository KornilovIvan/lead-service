package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

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

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	customers := service.NewCustomerService(repository.NewCustomerRepository(db))
	leads := service.NewLeadService(repository.NewLeadRepository(db))

	r := chi.NewRouter()
	handler.New(customers, leads).Register(r)

	log.Fatal(http.ListenAndServe(addr, r))
}
