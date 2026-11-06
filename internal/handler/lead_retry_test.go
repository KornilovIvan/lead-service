package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/mock/gomock"

	"lead-service/internal/bitrix"
	"lead-service/internal/bitrix/mock"
	"lead-service/internal/repository"
	"lead-service/internal/service"
	"lead-service/internal/testdb"
)

func TestRetryBitrixUnauthorized(t *testing.T) {
	db := testdb.Open(t)

	var customerID int
	if err := db.Get(&customerID, `INSERT INTO customers (name) VALUES ('retry-401') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	var leadID int
	if err := db.Get(&leadID, `
		INSERT INTO leads (customer_id, name, phone, status)
		VALUES ($1, 'retry', '79008880101', 'new')
		RETURNING id`, customerID); err != nil {
		t.Fatal(err)
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockClient(ctrl)
	client.EXPECT().
		FindByOriginID(gomock.Any(), strconv.Itoa(leadID)).
		Return("", false, &bitrix.HTTPError{Status: http.StatusUnauthorized})

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	h := New(nil, service.NewLeadService(repository.NewLeadRepository(db), client), nil, nil, nil)
	r := chi.NewRouter()
	h.Register(r)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/leads/"+strconv.Itoa(leadID)+"/retry", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "bitrix error" {
		t.Fatalf("error %q", body.Error)
	}
	if !strings.Contains(logs.String(), "webhook revoked") {
		t.Fatalf("log %q", logs.String())
	}
}
