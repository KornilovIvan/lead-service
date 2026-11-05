package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"lead-service/internal/model"
	"lead-service/internal/service"
)

type Handler struct {
	customers     *service.CustomerService
	leads         *service.LeadService
	imports       *service.ImportService
	qualification *service.QualificationService
	exports       *service.ExportService
}

func New(customers *service.CustomerService, leads *service.LeadService, imports *service.ImportService, qualification *service.QualificationService, exports *service.ExportService) *Handler {
	return &Handler{customers: customers, leads: leads, imports: imports, qualification: qualification, exports: exports}
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createCustomer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	customer, err := h.customers.Create(r.Context(), req.Name)
	if err == service.ErrNameRequired {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, customer)
}

func (h *Handler) getCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	customer, err := h.customers.Get(r.Context(), id)
	if err == service.ErrCustomerNotFound {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, customer)
}

func (h *Handler) createLead(w http.ResponseWriter, r *http.Request) {
	var req model.Lead
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	lead, err := h.leads.Create(r.Context(), req)
	switch err {
	case nil:
		writeJSON(w, http.StatusCreated, lead)
	case service.ErrCustomerNotFound,
		service.ErrInvalidStatus,
		service.ErrPhoneRequired,
		service.ErrInvalidPhone:
		writeError(w, http.StatusBadRequest, err.Error())
	case service.ErrDuplicateLead:
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *Handler) getLead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	lead, err := h.leads.Get(r.Context(), id)
	if err == service.ErrNotFound {
		writeError(w, http.StatusNotFound, "lead not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, lead)
}

func (h *Handler) listLeads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rawCustomerID := q.Get("customer_id")
	if rawCustomerID == "" {
		writeError(w, http.StatusBadRequest, service.ErrCustomerIDRequired.Error())
		return
	}

	customerID, err := strconv.Atoi(rawCustomerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, service.ErrCustomerIDRequired.Error())
		return
	}

	limit := 20
	if raw := q.Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, service.ErrInvalidLimit.Error())
			return
		}
	}

	var status *string
	if raw := q.Get("status"); raw != "" {
		status = &raw
	}

	rawAfterCreated := q.Get("after_created_at")
	rawAfterID := q.Get("after_id")
	if (rawAfterCreated == "") != (rawAfterID == "") {
		writeError(w, http.StatusBadRequest, service.ErrInvalidCursor.Error())
		return
	}

	var afterCreatedAt *time.Time
	var afterID *int
	if rawAfterCreated != "" {
		t, err := time.Parse(time.RFC3339Nano, rawAfterCreated)
		if err != nil {
			writeError(w, http.StatusBadRequest, service.ErrInvalidCursor.Error())
			return
		}
		afterCreatedAt = &t
		id, err := strconv.Atoi(rawAfterID)
		if err != nil {
			writeError(w, http.StatusBadRequest, service.ErrInvalidCursor.Error())
			return
		}
		afterID = &id
	}

	page, err := h.leads.List(r.Context(), customerID, limit, status, afterCreatedAt, afterID)
	if err == service.ErrCustomerIDRequired || err == service.ErrInvalidStatus || err == service.ErrInvalidCursor || err == service.ErrInvalidLimit {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}
