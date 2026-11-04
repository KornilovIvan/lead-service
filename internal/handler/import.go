package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"lead-service/internal/model"
	"lead-service/internal/service"
)

func (h *Handler) createImport(w http.ResponseWriter, r *http.Request) {
	var req model.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	job, err := h.imports.Create(r.Context(), req)
	switch err {
	case nil:
		writeJSON(w, http.StatusCreated, job)
	case service.ErrCustomerNotFound,
		service.ErrInvalidSource,
		service.ErrInvalidImport,
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

func (h *Handler) getImport(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	job, err := h.imports.Get(r.Context(), id)
	if err == service.ErrImportNotFound {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}
