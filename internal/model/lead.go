package model

import "time"

type Lead struct {
	ID         int       `db:"id" json:"id"`
	CustomerID int       `db:"customer_id" json:"customer_id"`
	Name       string    `db:"name" json:"name"`
	Phone      *string   `db:"phone" json:"phone"`
	Email      *string   `db:"email" json:"email"`
	Status     string    `db:"status" json:"status"`
	BitrixID   *string   `db:"bitrix_id" json:"bitrix_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	CRMLabel   string    `db:"-" json:"crm_label,omitempty"`
}

type LeadCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int       `json:"id"`
}

type LeadPage struct {
	Items []Lead      `json:"items"`
	Next  *LeadCursor `json:"next"`
}

type LeadPatch struct {
	Name   *string `json:"name"`
	Phone  *string `json:"phone"`
	Email  *string `json:"email"`
	Status *string `json:"status"`
}
