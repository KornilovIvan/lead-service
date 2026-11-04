package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"lead-service/internal/model"
	"lead-service/internal/repository"
)

type QualificationService struct {
	repo *repository.QualificationRepository
}

func NewQualificationService(repo *repository.QualificationRepository) *QualificationService {
	return &QualificationService{repo: repo}
}

func (s *QualificationService) Create(ctx context.Context, rule model.QualificationRule) (model.QualificationRule, error) {
	if rule.Field != "name" && rule.Field != "phone" && rule.Field != "email" {
		return model.QualificationRule{}, ErrInvalidField
	}
	if rule.ResultStatus != "qualified" && rule.ResultStatus != "rejected" {
		return model.QualificationRule{}, ErrInvalidStatus
	}

	rule.Pattern = strings.TrimSpace(rule.Pattern)
	if rule.Pattern == "" {
		return model.QualificationRule{}, ErrPatternRequired
	}

	created, err := s.repo.Create(ctx, rule)
	if isForeignKeyViolation(err) {
		return model.QualificationRule{}, ErrCustomerNotFound
	}
	return created, err
}

func (s *QualificationService) List(ctx context.Context, customerID int) ([]model.QualificationRule, error) {
	if customerID == 0 {
		return nil, ErrCustomerIDRequired
	}
	return s.repo.ListByCustomer(ctx, customerID)
}

func (s *QualificationService) Qualify(ctx context.Context, leadID int) (model.Lead, error) {
	lead, err := s.repo.GetLeadByID(ctx, leadID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Lead{}, ErrLeadNotFound
	}
	if err != nil {
		return model.Lead{}, err
	}

	rules, err := s.repo.ListByCustomer(ctx, lead.CustomerID)
	if err != nil {
		return model.Lead{}, err
	}

	for _, rule := range rules {
		value, ok := fieldValue(lead, rule.Field)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(value), strings.ToLower(rule.Pattern)) {
			if err := s.repo.UpdateLeadStatus(ctx, lead.ID, rule.ResultStatus); err != nil {
				return model.Lead{}, err
			}
			lead.Status = rule.ResultStatus
			return lead, nil
		}
	}

	return lead, nil
}

func fieldValue(lead model.Lead, field string) (string, bool) {
	switch field {
	case "name":
		return lead.Name, true
	case "phone":
		if lead.Phone == nil {
			return "", false
		}
		return *lead.Phone, true
	case "email":
		if lead.Email == nil {
			return "", false
		}
		return *lead.Email, true
	default:
		return "", false
	}
}
