package zypper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	userAgent  string
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		userAgent: "upkg-zypper/1.0",
	}
}

func (c *Client) Get(ctx context.Context, url string) (io.ReadCloser, error) {
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

	return resp.Body, nil
}

func (c *Client) Download(ctx context.Context, url string, w io.Writer) (int64, error) {
	body, err := c.Get(ctx, url)
	if err != nil {
		return 0, err
	}
	defer body.Close()

	return io.Copy(w, body)
}