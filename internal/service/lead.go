package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

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
	lead.Phone = nullIfEmpty(lead.Phone)

	if lead.Status == "" {
		lead.Status = "new"
	}
	if lead.Status != "new" && lead.Status != "qualified" && lead.Status != "rejected" {
		return model.Lead{}, ErrInvalidStatus
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
		return model.Lead{}, nil
	}
	return lead, err
}

func (s *LeadService) List(ctx context.Context, customerID, limit, offset int) ([]model.Lead, error) {
	if customerID == 0 {
		return nil, ErrCustomerIDRequired
	}
	return s.repo.ListByCustomer(ctx, customerID, limit, offset)
}

func nullIfEmpty(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	return v
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
