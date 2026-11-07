package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/testdb"
)

func leadCRMLabel(t *testing.T, h http.Handler, id int) string {
	t.Helper()
	rec := get(t, h, fmt.Sprintf("/api/v1/leads/%d", id))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	var body struct {
		CRMLabel string `json:"crm_label"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.CRMLabel
}

func setJobStatus(t *testing.T, db *sqlx.DB, leadID int, status string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE jobs SET status = $2 WHERE lead_id = $1`, leadID, status); err != nil {
		t.Fatal(err)
	}
}

func createLead(t *testing.T, h http.Handler, customerID int, name string) int {
	t.Helper()
	rec := postJSON(t, h, "/api/v1/leads", fmt.Sprintf(
		`{"customer_id":%d,"name":"%s","phone":"%s"}`, customerID, name, nextPhone()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func TestLeadCRMLabelHTTP(t *testing.T) {
	db := testdb.Open(t)
	customerID := insertCustomer(t, db, "label-single")
	router := newRouter(db)
	leadID := createLead(t, router, customerID, "Метка")

	// The queued job created with the lead is the latest one.
	if got := leadCRMLabel(t, router, leadID); got != "в CRM ещё нет" {
		t.Fatalf("queued label = %q, want %q", got, "в CRM ещё нет")
	}

	setJobStatus(t, db, leadID, "failed")
	if got := leadCRMLabel(t, router, leadID); got != "выгрузка упала" {
		t.Fatalf("failed label = %q, want %q", got, "выгрузка упала")
	}

	if _, err := db.Exec(`UPDATE leads SET bitrix_id = '42' WHERE id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if got := leadCRMLabel(t, router, leadID); got != "в CRM ушло" {
		t.Fatalf("bitrix label = %q, want %q", got, "в CRM ушло")
	}

	if _, err := db.Exec(`DELETE FROM jobs WHERE lead_id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE leads SET bitrix_id = NULL WHERE id = $1`, leadID); err != nil {
		t.Fatal(err)
	}
	if got := leadCRMLabel(t, router, leadID); got != "в CRM ещё нет" {
		t.Fatalf("no jobs label = %q, want %q", got, "в CRM ещё нет")
	}
}

func TestLeadCRMLabelListHTTP(t *testing.T) {
	db := testdb.Open(t)
	customerID := insertCustomer(t, db, "label-list")
	router := newRouter(db)

	ids := make([]int, 4)
	for i := range ids {
		ids[i] = createLead(t, router, customerID, fmt.Sprintf("Список%d", i))
	}

	// already in CRM, even though the latest job failed
	if _, err := db.Exec(`UPDATE leads SET bitrix_id = '1' WHERE id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	setJobStatus(t, db, ids[0], "failed")

	// no bitrix_id, latest job failed
	setJobStatus(t, db, ids[1], "failed")

	// no bitrix_id, latest job queued (default)
	// ids[2] stays queued

	// no jobs at all
	if _, err := db.Exec(`DELETE FROM jobs WHERE lead_id = $1`, ids[3]); err != nil {
		t.Fatal(err)
	}

	rec := get(t, router, fmt.Sprintf("/api/v1/leads?customer_id=%d&limit=10", customerID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	var page struct {
		Items []struct {
			ID       int    `json:"id"`
			CRMLabel string `json:"crm_label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}

	labels := make(map[int]string, len(page.Items))
	for _, item := range page.Items {
		labels[item.ID] = item.CRMLabel
	}

	want := map[int]string{
		ids[0]: "в CRM ушло",
		ids[1]: "выгрузка упала",
		ids[2]: "в CRM ещё нет",
		ids[3]: "в CRM ещё нет",
	}
	for id, w := range want {
		if labels[id] != w {
			t.Fatalf("lead %d label = %q, want %q", id, labels[id], w)
		}
	}
}
