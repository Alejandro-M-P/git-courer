package openai_standard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultRetryWait holds the exponential backoff durations for retries.
// Matches the current Ollama adapter: [10s, 20s, 30s, 60s, 90s].
var DefaultRetryWait = []time.Duration{
	10 * time.Second,
	20 * time.Second,
	30 * time.Second,
	60 * time.Second,
	90 * time.Second,
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) ClientOption {
	return func(c *Client) { c.maxRetries = n }
}

// WithRetryWait sets the retry backoff durations.
func WithRetryWait(waits []time.Duration) ClientOption {
	return func(c *Client) { c.retryWait = waits }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = httpClient }
}

// WithAPIKey sets an API key for Authorization header.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) { c.apiKey = key }
}

// Client is an HTTP client for OpenAI-compatible /v1/ endpoints
// with retry logic (exponential backoff on 429, 500, 503).
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
	retryWait  []time.Duration
}

// NewClient creates a new HTTP client for OpenAI-compatible endpoints.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
		maxRetries: 5,
		retryWait:  DefaultRetryWait,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Do sends an HTTP request with retry logic.
// Retries on 429, 500, and 503 status codes.
func (c *Client) Do(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var lastErr error
	retries := c.maxRetries

	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			wait := c.waitDuration(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		var reqBody io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshal request body: %w", err)
			}
			reqBody = bytes.NewBuffer(data)
		}

		url := c.baseURL + path
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		// Use timeout for this attempt
		timeout := timeoutFor(attempt)
		client := c.httpClient
		if client.Timeout < timeout {
			client = &http.Client{Timeout: timeout}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read response: %w", readErr)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 503 {
			lastErr = fmt.Errorf("server returned %d: %s", resp.StatusCode, truncate(string(respBody), 200))
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", retries, lastErr)
}

// Post sends a POST request.
func (c *Client) Post(ctx context.Context, path string, body interface{}) ([]byte, error) {
	return c.Do(ctx, "POST", path, body)
}

// Get sends a GET request.
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.Do(ctx, "GET", path, nil)
}

// waitDuration returns the backoff duration for the given attempt (0-indexed).
func (c *Client) waitDuration(attempt int) time.Duration {
	if attempt < len(c.retryWait) {
		return c.retryWait[attempt]
	}
	return c.retryWait[len(c.retryWait)-1]
}

// timeoutFor returns the timeout duration for the given attempt index,
// matching the current Ollama adapter behavior.
func timeoutFor(attempt int) time.Duration {
	switch {
	case attempt >= 4:
		return 180 * time.Second
	case attempt >= 2:
		return 120 * time.Second
	default:
		return 60 * time.Second
	}
}

// truncate shortens a string for error messages.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
