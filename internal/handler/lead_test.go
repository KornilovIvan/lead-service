package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"lead-service/internal/repository"
	"lead-service/internal/service"
	"lead-service/internal/testdb"
)

var phoneSeq atomic.Int64

func TestCreateLead(t *testing.T) {
	db := testdb.Open(t)
	customerID := insertCustomer(t, db, "lead-create")

	rec := postJSON(t, newRouter(db), "/api/v1/leads", fmt.Sprintf(
		`{"customer_id":%d,"name":"Иван","phone":"%s","status":"new"}`,
		customerID, nextPhone(),
	))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	var lead struct {
		ID         int     `json:"id"`
		CustomerID int     `json:"customer_id"`
		BitrixID   *string `json:"bitrix_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lead); err != nil {
		t.Fatal(err)
	}
	if lead.CustomerID != customerID {
		t.Fatalf("customer_id %d", lead.CustomerID)
	}
	if lead.BitrixID != nil {
		t.Fatalf("bitrix_id %v", lead.BitrixID)
	}
	var jobs int
	if err := db.Get(&jobs, `SELECT count(*) FROM jobs WHERE lead_id = $1 AND status = 'queued'`, lead.ID); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("jobs %d", jobs)
	}
}

func TestCreateLeadDuplicatePhone(t *testing.T) {
	db := testdb.Open(t)
	customerID := insertCustomer(t, db, "lead-dup")
	phone := nextPhone()

	body := fmt.Sprintf(`{"customer_id":%d,"name":"А","phone":"%s","status":"new"}`, customerID, phone)
	router := newRouter(db)
	first := postJSON(t, router, "/api/v1/leads", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status %d, body %s", first.Code, first.Body.String())
	}
	second := postJSON(t, router, "/api/v1/leads", body)
	if second.Code != http.StatusConflict {
		t.Fatalf("second status %d, body %s", second.Code, second.Body.String())
	}
}

func TestCreateLeadRequiresPhone(t *testing.T) {
	db := testdb.Open(t)
	customerID := insertCustomer(t, db, "lead-nophone")
	rec := postJSON(t, newRouter(db), "/api/v1/leads", fmt.Sprintf(
		`{"customer_id":%d,"name":"Без телефона","status":"new"}`,
		customerID,
	))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "phone is required") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestGetMissingLead(t *testing.T) {
	db := testdb.Open(t)
	rec := get(t, newRouter(db), "/api/v1/leads/99999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "lead not found") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func newRouter(db *sqlx.DB) http.Handler {
	h := New(nil, service.NewLeadService(repository.NewLeadRepository(db)), nil, nil, nil)
	r := chi.NewRouter()
	h.Register(r)
	return r
}

func insertCustomer(t *testing.T, db *sqlx.DB, name string) int {
	t.Helper()
	var id int
	if err := db.Get(&id, `INSERT INTO customers (name) VALUES ($1) RETURNING id`, name); err != nil {
		t.Fatal(err)
	}
	return id
}

func nextPhone() string {
	n := phoneSeq.Add(1)
	return fmt.Sprintf("79%09d", n)
}

func postJSON(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
