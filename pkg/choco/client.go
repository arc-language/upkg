// pkg/choco/client.go
package choco

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles HTTP requests to Chocolatey repository
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new Chocolatey repository HTTP client
func NewClient() *Client {
	return NewClientWithTimeout(2 * time.Minute)
}

// NewClientWithTimeout creates a new client with custom timeout
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
		userAgent: "upkg-choco/1.0",
	}
}

// Get performs an HTTP GET request
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/atom+xml,application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	return resp, nil
}

// Download downloads a file to the given writer
func (c *Client) Download(ctx context.Context, url string, w io.Writer) (int64, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	written, err := io.Copy(w, resp.Body)
	if err != nil {
		return written, fmt.Errorf("copying data: %w", err)
	}

	return written, nil
}