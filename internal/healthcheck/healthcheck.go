// Package healthcheck runs persisted project healthchecks through an injectable client.
package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Client performs one healthcheck request.
type Client interface {
	Do(*http.Request) (*http.Response, error)
}

// Result is the outcome used by runtime reconciliation.
type Result struct {
	StatusCode int
	Healthy    bool
}

// Engine evaluates one persisted healthcheck endpoint.
type Engine struct {
	Client Client
}

func New(client Client) Engine {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return Engine{Client: client}
}

func (e Engine) Check(ctx context.Context, endpoint string, expectedStatus int) (Result, error) {
	if expectedStatus == 0 {
		expectedStatus = http.StatusOK
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{}, err
	}
	client := e.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return Result{}, err
	}
	if response == nil || response.Body == nil {
		return Result{}, errors.New("healthcheck: client returned empty response")
	}
	defer response.Body.Close()
	return Result{StatusCode: response.StatusCode, Healthy: response.StatusCode == expectedStatus}, nil
}
