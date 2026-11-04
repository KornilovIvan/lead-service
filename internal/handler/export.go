package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"lead-service/internal/model"
	"lead-service/internal/service"
)

func (h *Handler) createExport(w http.ResponseWriter, r *http.Request) {
	var req model.ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	exp, err := h.exports.Create(r.Context(), req)
	if err == service.ErrInvalidExportKind || err == service.ErrLeadCustomerMismatch || err == service.ErrSheetsURLRequired || err == service.ErrEndpointRequired {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err == service.ErrLeadNotFound {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, exp)
}

func (h *Handler) listExports(w http.ResponseWriter, r *http.Request) {
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

	exports, err := h.exports.List(r.Context(), customerID)
	if err == service.ErrCustomerIDRequired {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exports)
}
