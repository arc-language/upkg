// pkg/winget/client.go
package winget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: APIBaseURL,
	}
}

// Search searches for packages by query string
func (c *Client) Search(ctx context.Context, query string) ([]PackageEntry, error) {
	// Endpoint: /v2/packages?query={query}&take=10
	u, _ := url.Parse(fmt.Sprintf("%s/packages", c.baseURL))
	q := u.Query()
	q.Set("query", query)
	q.Set("take", "20") // Limit results
	u.RawQuery = q.Encode()

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

	// The API returns a wrapper for search
	var result struct {
		Packages []PackageEntry `json:"Packages"`
		// API v2 sometimes returns array directly or inside a wrapper depending on the specific endpoint variant
		// We'll try to decode into the expected wrapper for /packages
	}

	// Note: winget.run v2 usually returns { "Packages": [...], "Total": ... }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return result.Packages, nil
}

// GetManifest fetches the full manifest for a specific package and version
func (c *Client) GetManifest(ctx context.Context, id, version string) (*Manifest, error) {
	// Endpoint: /v2/manifests/{id}/{version}
	// Note: version can be "latest" in some APIs, but winget.run usually requires specific version.
	// We'll try to use the version string provided.

	encodedID := url.PathEscape(id)
	encodedVersion := url.PathEscape(version)
	
	url := fmt.Sprintf("%s/manifests/%s/%s", c.baseURL, encodedID, encodedVersion)

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
		return nil, fmt.Errorf("manifest not found")
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