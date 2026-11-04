package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lead-service/internal/model"
	"lead-service/internal/repository"
)

type ExportService struct {
	repo           *repository.ExportRepository
	sheetsBaseURL  string
	webhookTimeout time.Duration
}

func NewExportService(repo *repository.ExportRepository, sheetsBaseURL, webhookTimeout string) *ExportService {
	timeout := 3 * time.Second
	if webhookTimeout != "" {
		if parsed, err := time.ParseDuration(webhookTimeout); err == nil {
			timeout = parsed
		}
	}
	return &ExportService{
		repo:           repo,
		sheetsBaseURL:  strings.TrimRight(sheetsBaseURL, "/"),
		webhookTimeout: timeout,
	}
}

func (s *ExportService) Create(ctx context.Context, req model.ExportRequest) (model.OutboundExport, error) {
	if req.Kind != "sheets" && req.Kind != "webhook" {
		return model.OutboundExport{}, ErrInvalidExportKind
	}

	endpoint := req.Endpoint
	switch req.Kind {
	case "sheets":
		if s.sheetsBaseURL == "" {
			return model.OutboundExport{}, ErrSheetsURLRequired
		}
		endpoint = s.sheetsBaseURL
	case "webhook":
		if strings.TrimSpace(req.Endpoint) == "" {
			return model.OutboundExport{}, ErrEndpointRequired
		}
		endpoint = req.Endpoint
	}

	lead, err := s.repo.GetLeadByID(ctx, req.LeadID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.OutboundExport{}, ErrLeadNotFound
	}
	if err != nil {
		return model.OutboundExport{}, err
	}
	if lead.CustomerID != req.CustomerID {
		return model.OutboundExport{}, ErrLeadCustomerMismatch
	}

	status := "done"
	if err := s.send(ctx, req.Kind, endpoint, lead); err != nil {
		status = "failed"
	}

	return s.repo.Create(ctx, model.OutboundExport{
		CustomerID: req.CustomerID,
		Kind:       req.Kind,
		Endpoint:   endpoint,
		Status:     status,
	})
}

func (s *ExportService) List(ctx context.Context, customerID int) ([]model.OutboundExport, error) {
	if customerID == 0 {
		return nil, ErrCustomerIDRequired
	}
	return s.repo.ListByCustomer(ctx, customerID)
}

func (s *ExportService) send(ctx context.Context, kind, endpoint string, lead model.Lead) error {
	url := endpoint
	timeout := s.webhookTimeout
	if kind == "sheets" {
		url = s.sheetsBaseURL + "/append"
		timeout = 10 * time.Second
	}

	body, err := json.Marshal(lead)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
