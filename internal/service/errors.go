package service

import "errors"

var (
	ErrNameRequired         = errors.New("name is required")
	ErrPhoneRequired        = errors.New("phone is required")
	ErrInvalidPhone         = errors.New("invalid phone")
	ErrCustomerNotFound     = errors.New("customer not found")
	ErrInvalidStatus        = errors.New("invalid status")
	ErrCustomerIDRequired   = errors.New("customer_id is required")
	ErrInvalidSource        = errors.New("invalid source")
	ErrInvalidImport        = errors.New("invalid import")
	ErrImportNotFound       = errors.New("import not found")
	ErrInvalidField         = errors.New("invalid field")
	ErrPatternRequired      = errors.New("pattern is required")
	ErrNotFound             = errors.New("lead not found")
	ErrLeadNotFound         = errors.New("lead not found")
	ErrInvalidExportKind    = errors.New("invalid export kind")
	ErrLeadCustomerMismatch = errors.New("lead customer mismatch")
	ErrSheetsURLRequired    = errors.New("sheets url is required")
	ErrEndpointRequired     = errors.New("endpoint is required")
)
