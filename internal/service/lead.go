package service

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"lead-service/internal/bitrix"
	"lead-service/internal/model"
	"lead-service/internal/repository"
)

type LeadService struct {
	repo   *repository.LeadRepository
	bitrix bitrix.Client
}

func NewLeadService(repo *repository.LeadRepository, bitrixClient bitrix.Client) *LeadService {
	return &LeadService{repo: repo, bitrix: bitrixClient}
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
	if isUniqueViolation(err) {
		return model.Lead{}, ErrDuplicateLead
	}
	if err != nil {
		return model.Lead{}, err
	}

	phone := ""
	if created.Phone != nil {
		phone = *created.Phone
	}
	email := ""
	if created.Email != nil {
		email = *created.Email
	}

	bitrixID, err := s.bitrix.AddLead(ctx, strconv.Itoa(created.ID), created.Name, phone, email)
	if err != nil {
		return created, nil
	}

	if err := s.repo.SetBitrixID(ctx, created.ID, bitrixID); err != nil {
		return model.Lead{}, err
	}
	created.BitrixID = &bitrixID
	return created, nil
}

func (s *LeadService) Get(ctx context.Context, id int) (model.Lead, error) {
	lead, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Lead{}, ErrNotFound
	}
	return lead, err
}

func (s *LeadService) List(ctx context.Context, customerID, limit int, status *string, afterCreatedAt *time.Time, afterID *int) (model.LeadPage, error) {
	if customerID == 0 {
		return model.LeadPage{}, ErrCustomerIDRequired
	}
	if limit < 1 || limit > 100 {
		return model.LeadPage{}, ErrInvalidLimit
	}
	if status != nil && *status != "new" && *status != "qualified" && *status != "rejected" {
		return model.LeadPage{}, ErrInvalidStatus
	}
	if (afterCreatedAt == nil) != (afterID == nil) {
		return model.LeadPage{}, ErrInvalidCursor
	}

	items, err := s.repo.ListByCustomer(ctx, customerID, status, afterCreatedAt, afterID, limit)
	if err != nil {
		return model.LeadPage{}, err
	}

	page := model.LeadPage{Items: items}
	if len(items) == limit {
		last := items[len(items)-1]
		page.Next = &model.LeadCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

func (s *LeadService) Update(ctx context.Context, id int, patch model.LeadPatch) (model.Lead, error) {
	lead, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Lead{}, ErrNotFound
	}
	if err != nil {
		return model.Lead{}, err
	}

	if patch.Name != nil {
		lead.Name = *patch.Name
	}
	if patch.Phone != nil {
		lead.Phone = patch.Phone
	}
	if patch.Email != nil {
		lead.Email = patch.Email
	}
	if patch.Status != nil {
		lead.Status = *patch.Status
	}

	lead, err = validateLead(lead)
	if err != nil {
		return model.Lead{}, err
	}

	updated, err := s.repo.Update(ctx, lead)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Lead{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.Lead{}, ErrDuplicateLead
	}
	return updated, err
}

func validateLead(lead model.Lead) (model.Lead, error) {
	phone, err := normalizePhone(lead.Phone)
	if err != nil {
		return model.Lead{}, err
	}
	lead.Phone = phone
	lead.Email = normalizeEmail(lead.Email)

	if lead.Status == "" {
		lead.Status = "new"
	}
	if lead.Status != "new" && lead.Status != "qualified" && lead.Status != "rejected" {
		return model.Lead{}, ErrInvalidStatus
	}
	return lead, nil
}
