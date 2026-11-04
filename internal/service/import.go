package service

import (
	"context"
	"database/sql"
	"errors"

	"lead-service/internal/model"
	"lead-service/internal/repository"
)

type ImportService struct {
	repo *repository.ImportRepository
}

func NewImportService(repo *repository.ImportRepository) *ImportService {
	return &ImportService{repo: repo}
}

func (s *ImportService) Create(ctx context.Context, req model.ImportRequest) (model.ImportJob, error) {
	if req.Source != "csv" && req.Source != "form" {
		return model.ImportJob{}, ErrInvalidSource
	}

	var leads []model.Lead
	var filename *string

	switch req.Source {
	case "form":
		if req.Lead == nil {
			return model.ImportJob{}, ErrInvalidImport
		}
		leads = []model.Lead{*req.Lead}
	case "csv":
		if req.Filename == "" || len(req.Leads) == 0 {
			return model.ImportJob{}, ErrInvalidImport
		}
		filename = &req.Filename
		leads = req.Leads
	}

	prepared := make([]model.Lead, 0, len(leads))
	for _, lead := range leads {
		lead, err := prepareImportedLead(lead, req.CustomerID)
		if err != nil {
			return model.ImportJob{}, err
		}
		prepared = append(prepared, lead)
	}

	job := model.ImportJob{
		CustomerID: req.CustomerID,
		Source:     req.Source,
		Filename:   filename,
	}

	created, err := s.repo.Create(ctx, job, prepared)
	if isForeignKeyViolation(err) {
		return model.ImportJob{}, ErrCustomerNotFound
	}
	return created, err
}

func (s *ImportService) Get(ctx context.Context, id int) (model.ImportJob, error) {
	job, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ImportJob{}, ErrImportNotFound
	}
	return job, err
}

func prepareImportedLead(lead model.Lead, customerID int) (model.Lead, error) {
	lead.CustomerID = customerID
	phone, err := requirePhone(lead.Phone)
	if err != nil {
		return model.Lead{}, err
	}
	lead.Phone = phone

	if lead.Status == "" {
		lead.Status = "new"
	}
	if lead.Status != "new" && lead.Status != "qualified" && lead.Status != "rejected" {
		return model.Lead{}, ErrInvalidStatus
	}
	return lead, nil
}
