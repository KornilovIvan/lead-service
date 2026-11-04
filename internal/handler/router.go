package handler

import "github.com/go-chi/chi/v5"

func (h *Handler) Register(r chi.Router) {
	r.Get("/health", h.health)
	r.Post("/api/v1/customers", h.createCustomer)
	r.Get("/api/v1/customers/{id}", h.getCustomer)
	r.Post("/api/v1/leads", h.createLead)
	r.Get("/api/v1/leads", h.listLeads)
	r.Get("/api/v1/leads/{id}", h.getLead)
	r.Post("/api/v1/imports", h.createImport)
	r.Get("/api/v1/imports/{id}", h.getImport)
}
