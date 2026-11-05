package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/model"
)

type QualificationRepository struct {
	db *sqlx.DB
}

func NewQualificationRepository(db *sqlx.DB) *QualificationRepository {
	return &QualificationRepository{db: db}
}

func (r *QualificationRepository) Create(ctx context.Context, rule model.QualificationRule) (model.QualificationRule, error) {
	const q = `
		INSERT INTO qualification_rules (customer_id, name, field, pattern, result_status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, customer_id, name, field, pattern, result_status`

	var created model.QualificationRule
	err := r.db.GetContext(ctx, &created, q, rule.CustomerID, rule.Name, rule.Field, rule.Pattern, rule.ResultStatus)
	return created, err
}

func (r *QualificationRepository) ListByCustomer(ctx context.Context, customerID int) ([]model.QualificationRule, error) {
	const q = `
		SELECT id, customer_id, name, field, pattern, result_status
		FROM qualification_rules
		WHERE customer_id = $1
		ORDER BY id ASC`

	rules := make([]model.QualificationRule, 0)
	err := r.db.SelectContext(ctx, &rules, q, customerID)
	return rules, err
}

func (r *QualificationRepository) GetLeadByID(ctx context.Context, id int) (model.Lead, error) {
	const q = `
		SELECT id, customer_id, name, phone, email, status, bitrix_id, created_at
		FROM leads
		WHERE id = $1`

	var lead model.Lead
	err := r.db.GetContext(ctx, &lead, q, id)
	return lead, err
}

func (r *QualificationRepository) UpdateLeadStatus(ctx context.Context, id int, status string) error {
	const q = `UPDATE leads SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, status, id)
	return err
}
