package trickest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Client provides access to the Trickest API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	vaultID    uuid.UUID

	Hive         *Service
	Orchestrator *Service
}

type Service struct {
	client   *Client
	basePath string
}

// NewClient creates a new Trickest API client
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    "https://api.trickest.io",
		httpClient: &http.Client{},
	}

	c.Hive = &Service{
		client:   c,
		basePath: "/hive/v1",
	}

	c.Orchestrator = &Service{
		client:   c,
		basePath: "/orchestrator/v1",
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.token == "" {
		return nil, fmt.Errorf("token is required")
	}

	if c.vaultID == uuid.Nil {
		user, err := c.GetCurrentUser(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get vault ID: %w", err)
		}
		c.vaultID = user.Profile.VaultInfo.ID
	}

	return c, nil
}

// Option configures a Client
type Option func(*Client)

// WithBaseURL sets the base URL for the client
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithToken sets the authentication token
func WithToken(token string) Option {
	return func(c *Client) {
		c.token = token
	}
}

// WithVaultID sets a specific vault ID
func WithVaultID(id uuid.UUID) Option {
	return func(c *Client) {
		c.vaultID = id
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// doJSON performs a JSON request to a service and decodes the response
func (s *Service) doJSON(ctx context.Context, method, path string, body, result any) error {
	fullPath := fmt.Sprintf("%s%s", s.basePath, path)

	return doJSON(ctx, s.client, method, fullPath, body, result)
}

// doRequest performs an HTTP request with common client settings
func doRequest(ctx context.Context, client *Client, method, path string, body any) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", client.baseURL, path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+client.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// checkResponseStatus checks the HTTP response status code and returns an error if the status is not successful (2xx).
// For 404 status, it returns a "resource not found" error.
// For other non-2xx statuses, it attempts to parse and return the API error details from the JSON response,
// falling back to a generic "unexpected status code: <status code>" error if parsing fails.
func checkResponseStatus(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("resource not found")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Details string `json:"details"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Details != "" {
			return fmt.Errorf("API error: %s", errResp.Details)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// decodeResponse decodes a JSON response into the provided value
func decodeResponse(resp *http.Response, v any) error {
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// doJSON performs a JSON request and decodes the response
func doJSON(ctx context.Context, client *Client, method, path string, body, result any) error {
	resp, err := doRequest(ctx, client, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := checkResponseStatus(resp); err != nil {
		return err
	}

	if result == nil {
		return nil
	}

	return decodeResponse(resp, result)
}
