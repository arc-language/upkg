// pkg/dnf/client.go
package dnf

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ulikunitz/xz"
)

// Client handles HTTP requests to Fedora repositories
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new Fedora repository HTTP client with default timeout
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
		userAgent: "upkg-dnf/1.0",
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
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	return resp, nil
}

// GetGzipped performs an HTTP GET request and returns a gzip reader
func (c *Client) GetGzipped(ctx context.Context, url string) (io.ReadCloser, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}

	return &combinedCloser{
		Reader:  gzReader,
		closers: []io.Closer{gzReader, resp.Body},
	}, nil
}

// GetXZ performs an HTTP GET request and returns an xz reader
func (c *Client) GetXZ(ctx context.Context, url string) (io.ReadCloser, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}

	xzReader, err := xz.NewReader(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("creating xz reader: %w", err)
	}

	return &combinedCloser{
		Reader:  xzReader,
		closers: []io.Closer{resp.Body},
	}, nil
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

// combinedCloser closes multiple closers
type combinedCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *combinedCloser) Close() error {
	var firstErr error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}