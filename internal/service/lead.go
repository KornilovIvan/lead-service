package service

import (
	"context"
	"database/sql"
	"errors"

	"lead-service/internal/model"
	"lead-service/internal/repository"
)

type LeadService struct {
	repo *repository.LeadRepository
}

func NewLeadService(repo *repository.LeadRepository) *LeadService {
	return &LeadService{repo: repo}
}

func (s *LeadService) Create(ctx context.Context, lead model.Lead) (model.Lead, error) {
	lead, err := validateLead(lead)
	if err != nil {
		return model.Lead{}, err
	}

	created, err := s.repo.Create(ctx, lead)
	if isForeignKeyViolation(err) {
		return model.Lead{}, ErrCustomerNotFound
	}
	return created, err
}

func (s *LeadService) Get(ctx context.Context, id int) (model.Lead, error) {
	lead, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Lead{}, ErrNotFound
	}
	return lead, err
}

func (s *LeadService) List(ctx context.Context, customerID, limit, offset int) ([]model.Lead, error) {
	if customerID == 0 {
		return nil, ErrCustomerIDRequired
	}
	return s.repo.ListByCustomer(ctx, customerID, limit, offset)
}

func validateLead(lead model.Lead) (model.Lead, error) {
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
