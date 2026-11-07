package repository

import (
	"context"
	"database/sql"
	"errors"
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
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.Lead{}, err
	}
	defer func() { _ = tx.Rollback() }()

	const insertLead = `
		INSERT INTO leads (customer_id, name, phone, email, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, customer_id, name, phone, email, status, bitrix_id, created_at`

	var created model.Lead
	if err := tx.GetContext(ctx, &created, insertLead, lead.CustomerID, lead.Name, lead.Phone, lead.Email, lead.Status); err != nil {
		return model.Lead{}, err
	}
	if err := enqueueJob(ctx, tx, created.ID); err != nil {
		return model.Lead{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Lead{}, err
	}
	return created, nil
}

func (r *LeadRepository) Enqueue(ctx context.Context, leadID int) error {
	return enqueueJob(ctx, r.db, leadID)
}

func (r *LeadRepository) GetByID(ctx context.Context, id int) (model.Lead, error) {
	const q = `
		SELECT id, customer_id, name, phone, email, status, bitrix_id, created_at
		FROM leads
		WHERE id = $1`

	var lead model.Lead
	err := r.db.GetContext(ctx, &lead, q, id)
	return lead, err
}

func (r *LeadRepository) ListByCustomer(ctx context.Context, customerID int, status *string, afterCreatedAt *time.Time, afterID *int, limit int) ([]model.Lead, error) {
	const q = `
		SELECT id, customer_id, name, phone, email, status, bitrix_id, created_at
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

func (r *LeadRepository) Update(ctx context.Context, lead model.Lead) (model.Lead, error) {
	const q = `
		UPDATE leads
		SET name = $1, phone = $2, email = $3, status = $4
		WHERE id = $5
		RETURNING id, customer_id, name, phone, email, status, bitrix_id, created_at`

	var updated model.Lead
	err := r.db.GetContext(ctx, &updated, q, lead.Name, lead.Phone, lead.Email, lead.Status, lead.ID)
	return updated, err
}

func enqueueJob(ctx context.Context, exec sqlx.ExtContext, leadID int) error {
	const q = `
		INSERT INTO jobs (lead_id, status, run_at)
		VALUES ($1, 'queued', now())`
	_, err := exec.ExecContext(ctx, q, leadID)
	return err
}

func (r *LeadRepository) SetBitrixID(ctx context.Context, id int, bitrixID string) error {
	const q = `UPDATE leads SET bitrix_id = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, bitrixID, id)
	return err
}

// LatestJobStatus returns the status of the job with the highest id for the
// lead. No jobs means (nil, nil).
func (r *LeadRepository) LatestJobStatus(ctx context.Context, leadID int) (*string, error) {
	const q = `
		SELECT status
		FROM jobs
		WHERE lead_id = $1
		ORDER BY id DESC
		LIMIT 1`

	var status string
	err := r.db.GetContext(ctx, &status, q, leadID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// LatestJobStatuses returns the status of the highest-id job per lead. Leads
// without jobs are simply absent from the map.
func (r *LeadRepository) LatestJobStatuses(ctx context.Context, leadIDs []int) (map[int]string, error) {
	out := make(map[int]string, len(leadIDs))
	if len(leadIDs) == 0 {
		return out, nil
	}

	q, args, err := sqlx.In(`
		SELECT DISTINCT ON (lead_id) lead_id, status
		FROM jobs
		WHERE lead_id IN (?)
		ORDER BY lead_id, id DESC`, leadIDs)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryxContext(ctx, r.db.Rebind(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var leadID int
		var status string
		if err := rows.Scan(&leadID, &status); err != nil {
			return nil, err
		}
		out[leadID] = status
	}
	return out, rows.Err()
}
