package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	if err == service.ErrCustomerNotFound || err == service.ErrInvalidStatus {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, lead)
}

func (h *Handler) getLead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	lead, err := h.leads.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
	}

	offset := 0
	if raw := q.Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
	}

	leads, err := h.leads.List(r.Context(), customerID, limit, offset)
	if err == service.ErrCustomerIDRequired {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, leads)
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
