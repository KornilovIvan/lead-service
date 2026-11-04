package service

import "errors"

var (
	ErrNameRequired       = errors.New("name is required")
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrCustomerIDRequired = errors.New("customer_id is required")
)
