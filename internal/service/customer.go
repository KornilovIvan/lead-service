package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"lead-service/internal/model"
	"lead-service/internal/repository"
)

type CustomerService struct {
	repo *repository.CustomerRepository
}

func NewCustomerService(repo *repository.CustomerRepository) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) Create(ctx context.Context, name string) (model.Customer, error) {
	name, err := validateCustomerName(name)
	if err != nil {
		return model.Customer{}, err
	}
	return s.repo.Create(ctx, name)
}

func (s *CustomerService) Get(ctx context.Context, id int) (model.Customer, error) {
	customer, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Customer{}, ErrCustomerNotFound
	}
	return customer, err
}

func validateCustomerName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNameRequired
	}
	return name, nil
}
