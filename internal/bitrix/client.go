package bitrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client interface {
	AddLead(ctx context.Context, originID, name, phone, email string) (bitrixID string, err error)
}

type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("bitrix status %d", e.Status)
}

type httpClient struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) Client {
	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
		http:    &http.Client{},
	}
}

func (c *httpClient) AddLead(ctx context.Context, originID, name, phone, email string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body := map[string]any{
		"fields": fields(originID, name, phone, email),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/crm.lead.add.json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Error("bitrix add lead", "lead_id", originID, "status", 0)
		return "", err
	}
	defer resp.Body.Close()
	slog.Info("bitrix add lead", "lead_id", originID, "status", resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &HTTPError{Status: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &HTTPError{Status: resp.StatusCode}
	}

	var parsed struct {
		Result int64 `json:"result"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", &HTTPError{Status: resp.StatusCode}
	}
	return strconv.FormatInt(parsed.Result, 10), nil
}

func fields(originID, name, phone, email string) map[string]any {
	out := map[string]any{
		"ORIGIN_ID": originID,
		"NAME":      name,
		"PHONE":     []map[string]string{{"VALUE": phone}},
	}
	if email != "" {
		out["EMAIL"] = []map[string]string{{"VALUE": email}}
	}
	return out
}
