package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"lead-service/internal/model"
)

type CustomerRepository struct {
	db *sqlx.DB
}

func NewCustomerRepository(db *sqlx.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(ctx context.Context, name string) (model.Customer, error) {
	const q = `
		INSERT INTO customers (name)
		VALUES ($1)
		RETURNING id, name, created_at`

	var customer model.Customer
	err := r.db.GetContext(ctx, &customer, q, name)
	return customer, err
}

func (r *CustomerRepository) GetByID(ctx context.Context, id int) (model.Customer, error) {
	const q = `
		SELECT id, name, created_at
		FROM customers
		WHERE id = $1`

	var customer model.Customer
	err := r.db.GetContext(ctx, &customer, q, id)
	return customer, err
}
