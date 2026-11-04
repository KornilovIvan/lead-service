package model

type QualificationRule struct {
	ID           int    `db:"id" json:"id"`
	CustomerID   int    `db:"customer_id" json:"customer_id"`
	Name         string `db:"name" json:"name"`
	Field        string `db:"field" json:"field"`
	Pattern      string `db:"pattern" json:"pattern"`
	ResultStatus string `db:"result_status" json:"result_status"`
}
