package model

import "time"

type ImportJob struct {
	ID         int       `db:"id" json:"id"`
	CustomerID int       `db:"customer_id" json:"customer_id"`
	Source     string    `db:"source" json:"source"`
	Filename   *string   `db:"filename" json:"filename"`
	Status     string    `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

type ImportRequest struct {
	CustomerID int    `json:"customer_id"`
	Source     string `json:"source"`
	Filename   string `json:"filename"`
	Lead       *Lead  `json:"lead"`
	Leads      []Lead `json:"leads"`
}
