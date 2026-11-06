package worker

import (
	"context"
	"errors"
	"net"
	"net/http"

	"lead-service/internal/bitrix"
)

type action int

const (
	// actionFailed marks the job as failed.
	actionFailed action = iota
	// actionRetry puts the job back into the queue with a backoff.
	actionRetry
	// actionWebhookRevoked marks the job as failed and logs the revoked webhook.
	actionWebhookRevoked
)

// classify decides what to do with a Bitrix sync error. This policy lives only
// in the worker: handlers and services know nothing about transport details.
func classify(err error) action {
	var httpErr *bitrix.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusUnauthorized:
			return actionWebhookRevoked
		case http.StatusRequestTimeout, http.StatusTooManyRequests:
			return actionRetry
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			return actionFailed
		default:
			return actionFailed
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return actionRetry
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return actionRetry
	}
	return actionFailed
}
