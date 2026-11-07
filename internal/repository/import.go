package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/model"
)

type ImportRepository struct {
	db *sqlx.DB
}

func NewImportRepository(db *sqlx.DB) *ImportRepository {
	return &ImportRepository{db: db}
}

func (r *ImportRepository) Create(ctx context.Context, job model.ImportJob, leads []model.Lead) (model.ImportJob, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return model.ImportJob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, lead := range leads {
		if _, err := insertLead(ctx, tx, lead); err != nil {
			return model.ImportJob{}, err
		}
	}

	created, err := insertImportJob(ctx, tx, job)
	if err != nil {
		return model.ImportJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ImportJob{}, err
	}
	return created, nil
}

func (r *ImportRepository) GetByID(ctx context.Context, id int) (model.ImportJob, error) {
	const q = `
		SELECT id, customer_id, source, filename, status, created_at
		FROM import_jobs
		WHERE id = $1`

	var job model.ImportJob
	err := r.db.GetContext(ctx, &job, q, id)
	return job, err
}

func insertLead(ctx context.Context, tx *sqlx.Tx, lead model.Lead) (model.Lead, error) {
	const q = `
		INSERT INTO leads (customer_id, name, phone, email, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, customer_id, name, phone, email, status, bitrix_id, created_at`

	var created model.Lead
	err := tx.GetContext(ctx, &created, q, lead.CustomerID, lead.Name, lead.Phone, lead.Email, lead.Status)
	return created, err
}

func insertImportJob(ctx context.Context, tx *sqlx.Tx, job model.ImportJob) (model.ImportJob, error) {
	const q = `
		INSERT INTO import_jobs (customer_id, source, filename, status)
		VALUES ($1, $2, $3, 'done')
		RETURNING id, customer_id, source, filename, status, created_at`

	var created model.ImportJob
	err := tx.GetContext(ctx, &created, q, job.CustomerID, job.Source, job.Filename)
	return created, err
}
