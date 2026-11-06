package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"lead-service/internal/testdb"
)

func TestRetryQueuesJob(t *testing.T) {
	db := testdb.Open(t)

	var customerID int
	if err := db.Get(&customerID, `INSERT INTO customers (name) VALUES ('retry-queue') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	var leadID int
	if err := db.Get(&leadID, `
		INSERT INTO leads (customer_id, name, phone, status)
		VALUES ($1, 'retry', $2, 'new')
		RETURNING id`, customerID, nextPhone()); err != nil {
		t.Fatal(err)
	}

	router := newRouter(db)
	first := postJSON(t, router, "/api/v1/leads/"+strconv.Itoa(leadID)+"/retry", "")
	if first.Code != http.StatusAccepted {
		t.Fatalf("status %d, body %s", first.Code, first.Body.String())
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "queued" {
		t.Fatalf("status %q", body.Status)
	}

	second := postJSON(t, router, "/api/v1/leads/"+strconv.Itoa(leadID)+"/retry", "")
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "already queued") {
		t.Fatalf("status %d, body %s", second.Code, second.Body.String())
	}
}

func TestRetryMissingLead(t *testing.T) {
	db := testdb.Open(t)
	rec := httptest.NewRecorder()
	newRouter(db).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/leads/99999/retry", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
}
