package service

import "errors"

var (
	ErrNameRequired       = errors.New("name is required")
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrInvalidStatus      = errors.New("invalid status")
	ErrCustomerIDRequired = errors.New("customer_id is required")
	ErrInvalidSource      = errors.New("invalid source")
	ErrInvalidImport      = errors.New("invalid import")
	ErrImportNotFound     = errors.New("import not found")
)
