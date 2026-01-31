// pkg/winget/manager.go
package winget

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config configures the Winget manager
type Config struct {
	InstallPath string
	CachePath   string
	Timeout     time.Duration
	Debug       bool
	Logger      *log.Logger
}

// IndexData represents the structure of the JSON file: Map[PackageID] -> List[Versions]
type IndexData map[string][]WingetVersion

type PackageManager struct {
	client     *Client
	config     *Config
	logger     *log.Logger
	httpClient *http.Client
	index      IndexData // In-memory package index
}

func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}
	
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	pm := &PackageManager{
		client: NewClient(cfg.Timeout, logger),
		config: cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		index: make(IndexData),
	}

	// Load the index from the cache file
	if err := pm.loadIndex(); err != nil {
		pm.logger.Printf("Warning: Failed to load Winget index: %v", err)
	}

	return pm
}

// loadIndex loads the JSON index file from the cache directory
func (pm *PackageManager) loadIndex() error {
	// The index is expected to be at: {CachePath}/index/winget_default.json
	indexPath := filepath.Join(pm.config.CachePath, "index", "winget_default.json")
	
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("index file does not exist at %s", indexPath)
	}

	f, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&pm.index); err != nil {
		return fmt.Errorf("parsing index json: %w", err)
	}
	
	pm.logger.Printf("Loaded %d packages from Winget index", len(pm.index))
	return nil
}

// Search exposes the client search functionality
func (pm *PackageManager) Search(ctx context.Context, query string) ([]PackageEntry, error) {
	return pm.client.Search(ctx, query)
}

// GetInfo retrieves package manifest details from API
func (pm *PackageManager) GetInfo(ctx context.Context, name string) (*Manifest, error) {
	// 1. Try direct lookup first
	if strings.Contains(name, ".") {
		entry, err := pm.client.GetPackage(ctx, name)
		if err == nil {
			// Try to resolve a working version
			return pm.fetchFirstValidManifest(ctx, entry, "latest")
		}
	}

	// 2. Fallback to Search
	results, err := pm.client.Search(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("package not found")
	}
	
	entry := pm.selectBestMatch(results, name)
	return pm.fetchFirstValidManifest(ctx, entry, "latest")
}

// Download downloads and installs a package using the LOCAL JSON index
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	pm.logger.Printf("Looking up package in local database: %s", opts.Package)

	if len(pm.index) == 0 {
		return fmt.Errorf("winget index not loaded (check cache directory)")
	}

	// Look up package in loaded index
	versionsRaw, exists := pm.index[opts.Package]
	if !exists {
		return fmt.Errorf("package %s not found in local database", opts.Package)
	}

	pm.logger.Printf("✓ Found package: %s with %d versions", opts.Package, len(versionsRaw))

	// Find the requested version
	var targetDownloads []WingetDownload
	var targetVerStr string

	if opts.Version != "" && opts.Version != "latest" {
		for _, v := range versionsRaw {
			if v.Version == opts.Version {
				targetDownloads = v.Downloads
				targetVerStr = v.Version
				break
			}
		}
		if targetDownloads == nil {
			return fmt.Errorf("version %s not found for package %s", opts.Version, opts.Package)
		}
	} else {
		// Use the first version (assumed to be latest/first in list coming from Python script)
		if len(versionsRaw) == 0 {
			return fmt.Errorf("no versions available for package %s", opts.Package)
		}
		targetDownloads = versionsRaw[0].Downloads
		targetVerStr = versionsRaw[0].Version
	}

	pm.logger.Printf("Using version: %s", targetVerStr)

	// Determine architecture
	arch := opts.Architecture
	if arch == "" {
		arch = runtime.GOARCH
		if mapped, ok := SupportedArchitectures[arch]; ok {
			arch = mapped
		}
	}

	pm.logger.Printf("Target architecture: %s", arch)

	// Find matching download
	var download *WingetDownload
	for i := range targetDownloads {
		dl := &targetDownloads[i]
		pm.logger.Printf("Available download: Arch=%s, Type=%s, URL=%s", dl.Arch, dl.Type, dl.URL)
		
		if strings.EqualFold(dl.Arch, arch) {
			download = dl
			break
		}
	}

	// If no exact match, try fallbacks
	if download == nil {
		pm.logger.Printf("No exact architecture match, looking for alternatives...")
		for i := range targetDownloads {
			dl := &targetDownloads[i]
			// For x64 systems, try x86 as fallback
			if arch == "x64" && strings.EqualFold(dl.Arch, "x86") {
				download = dl
				pm.logger.Printf("Using x86 build as fallback")
				break
			}
		}
	}

	// Last resort: use first available if list exists
	if download == nil && len(targetDownloads) > 0 {
		download = &targetDownloads[0]
		pm.logger.Printf("Using first available download: %s", download.Arch)
	}

	if download == nil {
		return fmt.Errorf("no suitable installer found for architecture: %s", arch)
	}

	pm.logger.Printf("Selected installer: %s (%s)", download.Type, download.Arch)
	pm.logger.Printf("Download URL: %s", download.URL)

	// Ensure directories exist
	if err := os.MkdirAll(pm.config.InstallPath, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}
	if err := os.MkdirAll(pm.config.CachePath, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// Determine file extension and name
	ext := determineExtensionFromURL(download.URL, download.Type)
	fileName := fmt.Sprintf("%s-%s.%s", sanitizeFilename(opts.Package), targetVerStr, ext)
	
	cachePath := filepath.Join(pm.config.CachePath, "downloads", fileName)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	pm.logger.Printf("Downloading to: %s", cachePath)

	// Download the file
	if err := pm.downloadFile(ctx, download.URL, cachePath); err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}

	pm.logger.Printf("✓ Download completed: %s", cachePath)

	// Handle extraction based on installer type
	installDir := filepath.Join(pm.config.InstallPath, opts.Package)
	
	isZip := strings.EqualFold(download.Type, "zip") || strings.HasSuffix(strings.ToLower(download.URL), ".zip")
	
	if opts.Extract && isZip {
		pm.logger.Printf("Extracting ZIP to: %s", installDir)
		if err := unzip(cachePath, installDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
		pm.logger.Printf("✓ Extracted to: %s", installDir)
	} else {
		// For non-zip or non-extract, just copy to install directory
		pm.logger.Printf("Installing to: %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("creating install directory: %w", err)
		}
		
		finalPath := filepath.Join(installDir, fileName)
		if err := copyFile(cachePath, finalPath); err != nil {
			return fmt.Errorf("copying file: %w", err)
		}
		
		// Make executable if it's a portable app
		if strings.EqualFold(download.Type, "portable") || strings.EqualFold(download.Type, "exe") {
			os.Chmod(finalPath, 0755)
		}
		
		pm.logger.Printf("✓ Installed to: %s", finalPath)
	}

	// Clean up archive if requested
	if !opts.KeepArchive && opts.Extract {
		pm.logger.Printf("Removing archive: %s", cachePath)
		os.Remove(cachePath)
	}

	return nil
}

// downloadFile downloads a file from URL to the specified path
func (pm *PackageManager) downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := pm.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// fetchFirstValidManifest iterates through available versions to find a working one
func (pm *PackageManager) fetchFirstValidManifest(ctx context.Context, entry *PackageEntry, requestedVer string) (*Manifest, error) {
	versions := []string{}
	
	// Priority 1: Latest.Version (if exists)
	if entry.Latest.Version != "" {
		versions = append(versions, entry.Latest.Version)
	}
	
	// Priority 2: Versions List (Raw)
	apiVersions := entry.GetVersions()
	versions = append(versions, apiVersions...)
	
	// Deduplicate
	seen := map[string]bool{}
	unique := []string{}
	for _, v := range versions {
		if v != "" && !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}
	
	// Try each version
	for _, v := range unique {
		pm.logger.Printf("Fetching manifest for version: %s", v)
		m, err := pm.client.GetManifest(ctx, entry.ID, v)
		if err == nil {
			pm.logger.Printf("✓ Successfully resolved version: %s", v)
			return m, nil
		}
		pm.logger.Printf("⚠ Version %s failed: %v", v, err)
	}
	
	// Final fallback: literal "latest"
	pm.logger.Printf("Trying literal 'latest'...")
	m, err := pm.client.GetManifest(ctx, entry.ID, "latest")
	if err == nil {
		return m, nil
	}
	
	return nil, fmt.Errorf("unable to resolve any valid manifest for %s", entry.ID)
}

// Helpers

func (pm *PackageManager) selectBestMatch(results []PackageEntry, query string) *PackageEntry {
	// 1. Exact ID Match
	for _, p := range results {
		if strings.EqualFold(p.ID, query) {
			return &p
		}
	}
	// 2. Exact Name Match
	for _, p := range results {
		if strings.EqualFold(p.Latest.Name, query) {
			return &p
		}
	}
	// 3. Suffix Match
	suffix := "." + strings.ToLower(query)
	for _, p := range results {
		if strings.HasSuffix(strings.ToLower(p.ID), suffix) {
			return &p
		}
	}
	return &results[0]
}

func determineExtensionFromURL(url, installerType string) string {
	u := strings.ToLower(url)
	if strings.Contains(u, ".zip") {
		return "zip"
	}
	if strings.Contains(u, ".msi") {
		return "msi"
	}
	if strings.Contains(u, ".msix") {
		return "msix"
	}
	if strings.Contains(u, ".exe") {
		return "exe"
	}
	
	// Fallback to installer type
	switch strings.ToLower(installerType) {
	case "zip":
		return "zip"
	case "msi":
		return "msi"
	case "msix":
		return "msix"
	case "exe":
		return "exe"
	default:
		return "bin"
	}
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return name
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0755)
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		
		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}
		
		out, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		
		if err != nil {
			return err
		}
	}
	return nil
}