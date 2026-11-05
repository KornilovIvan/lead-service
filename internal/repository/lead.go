package repository

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/model"
)

type LeadRepository struct {
	db *sqlx.DB
}

func NewLeadRepository(db *sqlx.DB) *LeadRepository {
	return &LeadRepository{db: db}
}

func (r *LeadRepository) Create(ctx context.Context, lead model.Lead) (model.Lead, error) {
	const q = `
		INSERT INTO leads (customer_id, name, phone, email, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, customer_id, name, phone, email, status, created_at`

	var created model.Lead
	err := r.db.GetContext(ctx, &created, q, lead.CustomerID, lead.Name, lead.Phone, lead.Email, lead.Status)
	return created, err
}

func (r *LeadRepository) GetByID(ctx context.Context, id int) (model.Lead, error) {
	const q = `
		SELECT id, customer_id, name, phone, email, status, created_at
		FROM leads
		WHERE id = $1`

	var lead model.Lead
	err := r.db.GetContext(ctx, &lead, q, id)
	return lead, err
}

func (r *LeadRepository) ListByCustomer(ctx context.Context, customerID int, status *string, afterCreatedAt *time.Time, afterID *int, limit int) ([]model.Lead, error) {
	const q = `
		SELECT id, customer_id, name, phone, email, status, created_at
		FROM leads
		WHERE customer_id = $1
		  AND ($2::text IS NULL OR status = $2)
		  AND (
		        $3::timestamptz IS NULL
		        OR created_at < $3
		        OR (created_at = $3 AND id < $4)
		      )
		ORDER BY created_at DESC, id DESC
		LIMIT $5`

	leads := make([]model.Lead, 0)
	err := r.db.SelectContext(ctx, &leads, q, customerID, status, afterCreatedAt, afterID, limit)
	return leads, err
}
