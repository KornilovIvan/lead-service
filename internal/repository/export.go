package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/model"
)

type ExportRepository struct {
	db *sqlx.DB
}

func NewExportRepository(db *sqlx.DB) *ExportRepository {
	return &ExportRepository{db: db}
}

func (r *ExportRepository) Create(ctx context.Context, exp model.OutboundExport) (model.OutboundExport, error) {
	const q = `
		INSERT INTO outbound_exports (customer_id, kind, endpoint, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, customer_id, kind, endpoint, status, created_at`

	var created model.OutboundExport
	err := r.db.GetContext(ctx, &created, q, exp.CustomerID, exp.Kind, exp.Endpoint, exp.Status)
	return created, err
}

func (r *ExportRepository) ListByCustomer(ctx context.Context, customerID int) ([]model.OutboundExport, error) {
	const q = `
		SELECT id, customer_id, kind, endpoint, status, created_at
		FROM outbound_exports
		WHERE customer_id = $1
		ORDER BY created_at DESC, id DESC`

	exports := make([]model.OutboundExport, 0)
	err := r.db.SelectContext(ctx, &exports, q, customerID)
	return exports, err
}

func (r *ExportRepository) GetLeadByID(ctx context.Context, id int) (model.Lead, error) {
	const q = `
		SELECT id, customer_id, name, phone, email, status, created_at
		FROM leads
		WHERE id = $1`

	var lead model.Lead
	err := r.db.GetContext(ctx, &lead, q, id)
	return lead, err
}
