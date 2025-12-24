// Package forwarder provides functionality to forward HTTP requests with retry logic.
package forwarder

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Forwarder executes upstream requests with retry logic
type Forwarder struct {
	client *http.Client
}

// New creates a new forwarder
func New(timeout time.Duration) *Forwarder {
	return &Forwarder{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Do executes an HTTP request with retry logic
func (f *Forwarder) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("upstream error: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
