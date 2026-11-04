package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"lead-service/internal/model"
	"lead-service/internal/service"
)

func (h *Handler) createQualificationRule(w http.ResponseWriter, r *http.Request) {
	var req model.QualificationRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	rule, err := h.qualification.Create(r.Context(), req)
	if err == service.ErrInvalidField || err == service.ErrInvalidStatus || err == service.ErrPatternRequired || err == service.ErrCustomerNotFound {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) listQualificationRules(w http.ResponseWriter, r *http.Request) {
	rawCustomerID := r.URL.Query().Get("customer_id")
	if rawCustomerID == "" {
		writeError(w, http.StatusBadRequest, service.ErrCustomerIDRequired.Error())
		return
	}

	customerID, err := strconv.Atoi(rawCustomerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, service.ErrCustomerIDRequired.Error())
		return
	}

	rules, err := h.qualification.List(r.Context(), customerID)
	if err == service.ErrCustomerIDRequired {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) qualifyLead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	lead, err := h.qualification.Qualify(r.Context(), id)
	if err == service.ErrLeadNotFound {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, lead)
}
