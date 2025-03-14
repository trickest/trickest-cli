package trickest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// Client provides access to the Trickest API
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	vaultID    uuid.UUID
	apiVersion string
}

// NewClient creates a new Trickest API client
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    "https://api.trickest.io/hive/",
		httpClient: &http.Client{},
		apiVersion: "v1",
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
		c.baseURL = url
	}
}

// WithAPIVersion sets the API version
func WithAPIVersion(version string) Option {
	return func(c *Client) {
		c.apiVersion = version
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

// doRequest performs an HTTP request with common client settings
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	url := fmt.Sprintf("%s%s%s", c.baseURL, c.apiVersion, path)

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

	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// decodeResponse decodes a JSON response into the provided value
func (c *Client) decodeResponse(resp *http.Response, v any) error {
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("resource not found")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// doJSON performs a JSON request and decodes the response
func (c *Client) doJSON(ctx context.Context, method, path string, body, result any) error {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	return c.decodeResponse(resp, result)
}
