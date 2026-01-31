// pkg/winget/manager.go
package winget

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
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

type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
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

	return &PackageManager{
		client: NewClient(cfg.Timeout, logger),
		config: cfg,
		logger: logger,
	}
}

// Search exposes the client search functionality
func (pm *PackageManager) Search(ctx context.Context, query string) ([]PackageEntry, error) {
	return pm.client.Search(ctx, query)
}

// GetInfo retrieves package manifest details
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

func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	pm.logger.Printf("Resolving package: %s", opts.Package)

	var entry *PackageEntry
	var err error

	// 1. Resolve Package Entry
	// Strategy A: Direct ID Lookup
	if strings.Contains(opts.Package, ".") {
		pm.logger.Printf("Attempting direct ID lookup for '%s'...", opts.Package)
		entry, err = pm.client.GetPackage(ctx, opts.Package)
		if err == nil {
			pm.logger.Printf("✓ Found package via ID: %s", entry.ID)
		} else {
			pm.logger.Printf("Direct lookup failed: %v", err)
		}
	}

	// Strategy B: Search Fallback
	if entry == nil {
		pm.logger.Printf("Falling back to search for '%s'...", opts.Package)
		results, err := pm.client.Search(ctx, opts.Package)
		
		if (err != nil || len(results) == 0) && strings.Contains(opts.Package, ".") {
			relaxed := strings.ReplaceAll(opts.Package, ".", " ")
			pm.logger.Printf("Retrying search with relaxed query: '%s'...", relaxed)
			results, err = pm.client.Search(ctx, relaxed)
		}

		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("package '%s' not found", opts.Package)
		}

		entry = pm.selectBestMatch(results, opts.Package)
		pm.logger.Printf("Selected from search: %s", entry.ID)
	}

	// 2. Resolve Manifest (Iterate versions if specific version not requested)
	var manifest *Manifest
	
	if opts.Version != "" && opts.Version != "latest" {
		// Specific version requested
		pm.logger.Printf("Fetching specific version: %s", opts.Version)
		manifest, err = pm.client.GetManifest(ctx, entry.ID, opts.Version)
		if err != nil {
			return fmt.Errorf("failed to fetch version %s: %w", opts.Version, err)
		}
	} else {
		// Auto-detect version
		manifest, err = pm.fetchFirstValidManifest(ctx, entry, "latest")
		if err != nil {
			return err
		}
	}

	// 3. Select Installer
	targetArch := opts.Architecture
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	wingetArch, ok := SupportedArchitectures[targetArch]
	if !ok {
		wingetArch = targetArch 
	}

	var bestInstaller *Installer
	for _, inst := range manifest.Installers {
		if strings.EqualFold(inst.Architecture, wingetArch) {
			bestInstaller = &inst
			break
		}
		if wingetArch == "x64" && strings.EqualFold(inst.Architecture, "x86") {
			if bestInstaller == nil {
				bestInstaller = &inst
			}
		}
	}

	if bestInstaller == nil {
		return fmt.Errorf("no compatible installer found for architecture %s", wingetArch)
	}

	pm.logger.Printf("Selected installer: %s (%s)", bestInstaller.InstallerType, bestInstaller.Architecture)

	// 4. Download
	ext := determineExtension(bestInstaller)
	fileName := fmt.Sprintf("%s-%s.%s", manifest.PackageName, manifest.PackageVersion, ext)
	fileName = sanitizeFilename(fileName)
	
	destPath := filepath.Join(pm.config.CachePath, "downloads", fileName)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	pm.logger.Printf("Downloading: %s", bestInstaller.InstallerUrl)
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	
	if err := pm.client.DownloadFile(ctx, bestInstaller.InstallerUrl, f); err != nil {
		f.Close()
		return fmt.Errorf("download error: %w", err)
	}
	f.Close()

	// 5. Verify Hash
	if opts.VerifyHash && bestInstaller.InstallerSha256 != "" {
		pm.logger.Printf("Verifying hash...")
		if err := verifyHash(destPath, bestInstaller.InstallerSha256); err != nil {
			return err
		}
	}

	// 6. Install
	installDir := filepath.Join(pm.config.InstallPath, manifest.PackageIdentifier)
	
	isZip := bestInstaller.InstallerType == InstallerTypeZip || strings.HasSuffix(strings.ToLower(bestInstaller.InstallerUrl), ".zip")
	
	if opts.Extract && isZip {
		pm.logger.Printf("Extracting to %s", installDir)
		if err := unzip(destPath, installDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		pm.logger.Printf("Installing to %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return err
		}
		finalPath := filepath.Join(installDir, fileName)
		input, _ := os.ReadFile(destPath)
		os.WriteFile(finalPath, input, 0755)
		
		pm.logger.Printf("✓ Package installed to: %s", finalPath)
	}

	if !opts.KeepArchive {
		os.Remove(destPath)
	}

	return nil
}

// fetchFirstValidManifest iterates through available versions to find a working one
func (pm *PackageManager) fetchFirstValidManifest(ctx context.Context, entry *PackageEntry, requestedVer string) (*Manifest, error) {
	// If the user asked for a specific version that isn't "latest", we shouldn't be here (handled in Download),
	// but for GetInfo we might need this.
	
	versions := []string{}
	
	// Priority 1: Latest.Version (if exists)
	if entry.Latest.Version != "" {
		versions = append(versions, entry.Latest.Version)
	}
	
	// Priority 2: Versions List (Raw)
	// The API typically returns versions descending (newest first) or ascending.
	// We'll try them in the order provided by the API, assuming the API puts relevant ones first.
	// If "Latest.Version" was missing, Versions[0] is our best bet.
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
		if strings.EqualFold(p.ID, query) { return &p }
	}
	// 2. Exact Name Match
	for _, p := range results {
		if strings.EqualFold(p.Latest.Name, query) { return &p }
	}
	// 3. Suffix Match
	suffix := "." + strings.ToLower(query)
	for _, p := range results {
		if strings.HasSuffix(strings.ToLower(p.ID), suffix) { return &p }
	}
	return &results[0]
}

func determineExtension(i *Installer) string {
	u := strings.ToLower(i.InstallerUrl)
	if strings.Contains(u, ".zip") { return "zip" }
	if strings.Contains(u, ".msi") { return "msi" }
	if strings.Contains(u, ".msix") { return "msix" }
	if strings.Contains(u, ".exe") { return "exe" }
	return "bin"
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

func verifyHash(path, expected string) error {
	f, err := os.Open(path)
	if err != nil { return err }
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil { return err }
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), expected) {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil { return err }
	defer r.Close()
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) { continue }
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil { return err }
		out, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil { return err }
		rc, err := f.Open()
		if err != nil { out.Close(); return err }
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}
	return nil
}