package model

import "time"

type OutboundExport struct {
	ID         int       `db:"id" json:"id"`
	CustomerID int       `db:"customer_id" json:"customer_id"`
	Kind       string    `db:"kind" json:"kind"`
	Endpoint   string    `db:"endpoint" json:"endpoint"`
	Status     string    `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

type ExportRequest struct {
	CustomerID int    `json:"customer_id"`
	LeadID     int    `json:"lead_id"`
	Kind       string `json:"kind"`
	Endpoint   string `json:"endpoint"`
}
