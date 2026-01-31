// pkg/winget/client.go
package winget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     *log.Logger
}

func NewClient(timeout time.Duration, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: APIBaseURL,
		logger:  logger,
	}
}

// Search searches for packages by query string
func (c *Client) Search(ctx context.Context, query string) ([]PackageEntry, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/packages", c.baseURL))
	q := u.Query()
	q.Set("query", query)
	q.Set("take", "20")
	u.RawQuery = q.Encode()

	c.logger.Printf("[Winget API] Searching: %s", u.String())

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Packages []PackageEntry `json:"Packages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return result.Packages, nil
}

// GetManifest fetches the full manifest for a specific package and version
func (c *Client) GetManifest(ctx context.Context, id, version string) (*Manifest, error) {
	encodedID := url.PathEscape(id)
	encodedVersion := url.PathEscape(version)
	
	url := fmt.Sprintf("%s/manifests/%s/%s", c.baseURL, encodedID, encodedVersion)
	c.logger.Printf("[Winget API] Fetching Manifest: %s", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("manifest not found for %s @ %s", id, version)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	return &manifest, nil
}

// DownloadFile downloads a file from a URL to a writer
func (c *Client) DownloadFile(ctx context.Context, url string, w io.Writer) error {
	c.logger.Printf("[Winget API] Downloading File: %s", url)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	_, err = io.Copy(w, resp.Body)
	return err
}