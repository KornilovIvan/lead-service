package model

import "time"

type Lead struct {
	ID         int       `db:"id" json:"id"`
	CustomerID int       `db:"customer_id" json:"customer_id"`
	Name       string    `db:"name" json:"name"`
	Phone      *string   `db:"phone" json:"phone"`
	Email      *string   `db:"email" json:"email"`
	Status     string    `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
