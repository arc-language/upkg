// client.go
package nix

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles HTTP requests to Nix services
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new Nix HTTP client with default timeout
func NewClient() *Client {
	return NewClientWithTimeout(30 * time.Second)
}

// NewClientWithTimeout creates a new Nix HTTP client with custom timeout
func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		userAgent: "arc-lang/1.0",
	}
}

// Get performs an HTTP GET request
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return resp, nil
}

// Download downloads a file to the given writer
func (c *Client) Download(ctx context.Context, url string, w io.Writer) error {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	return err
}

// GetString fetches a URL and returns the body as a string
func (c *Client) GetString(ctx context.Context, url string) (string, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	return string(body), nil
}