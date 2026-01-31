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

func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	pm.logger.Printf("Resolving package: %s", opts.Package)

	id := opts.Package
	version := opts.Version
	
	// 1. Resolve Package ID and Version
	if version == "" || version == "latest" {
		var entry *PackageEntry
		var err error

		// Strategy A: Direct ID Lookup (Preferred)
		// If input looks like an ID (Publisher.Package), try to fetch it directly.
		if strings.Contains(opts.Package, ".") {
			pm.logger.Printf("Attempting direct ID lookup for '%s'...", opts.Package)
			entry, err = pm.client.GetPackage(ctx, opts.Package)
			if err == nil {
				pm.logger.Printf("✓ Found package via ID: %s", entry.ID)
			} else {
				pm.logger.Printf("Direct lookup failed: %v", err)
			}
		}

		// Strategy B: Search (Fallback)
		// If direct lookup failed or input is a keyword (no dot), perform a search.
		if entry == nil {
			pm.logger.Printf("Falling back to search for '%s'...", opts.Package)
			results, err := pm.client.Search(ctx, opts.Package)
			if err != nil {
				return fmt.Errorf("searching for package: %w", err)
			}
			if len(results) == 0 {
				return fmt.Errorf("package '%s' not found", opts.Package)
			}

			// Select best match from search results
			entry = pm.selectBestMatch(results, opts.Package)
			pm.logger.Printf("Selected from search: %s", entry.ID)
		}

		id = entry.ID
		
		// Version Selection
		// Priority: Latest.Version > Last in Versions list > "latest" literal
		if entry.Latest.Version != "" {
			version = entry.Latest.Version
		} else if len(entry.Versions) > 0 {
			version = entry.Versions[len(entry.Versions)-1]
		} else {
			version = "latest"
		}
		pm.logger.Printf("Resolved Version: %s", version)
	}

	// 2. Get Manifest
	manifest, err := pm.client.GetManifest(ctx, id, version)
	if err != nil {
		if version != "latest" {
			// If resolved version fails, try "latest" literal as last resort
			pm.logger.Printf("Failed to get version %s, retrying with 'latest'...", version)
			manifest, err = pm.client.GetManifest(ctx, id, "latest")
			if err != nil {
				return fmt.Errorf("fetching manifest: %w", err)
			}
		} else {
			return fmt.Errorf("fetching manifest: %w", err)
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
		// Fallback: x64 can run x86 on Windows
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

	pm.logger.Printf("Downloading from: %s", bestInstaller.InstallerUrl)
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	
	if err := pm.client.DownloadFile(ctx, bestInstaller.InstallerUrl, f); err != nil {
		f.Close()
		return fmt.Errorf("downloading file: %w", err)
	}
	f.Close()

	// 5. Verify Hash
	if opts.VerifyHash && bestInstaller.InstallerSha256 != "" {
		pm.logger.Printf("Verifying hash...")
		if err := verifyHash(destPath, bestInstaller.InstallerSha256); err != nil {
			return err
		}
	}

	// 6. Install / Extract
	installDir := filepath.Join(pm.config.InstallPath, manifest.PackageIdentifier)
	
	// Determine if we should extract based on installer type
	isZip := bestInstaller.InstallerType == InstallerTypeZip || strings.HasSuffix(strings.ToLower(bestInstaller.InstallerUrl), ".zip")
	
	if opts.Extract && isZip {
		pm.logger.Printf("Extracting zip to %s", installDir)
		if err := unzip(destPath, installDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		// For non-zips (exe, msi) or if extract is false, move the file
		pm.logger.Printf("Moving installer to %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return err
		}
		finalPath := filepath.Join(installDir, fileName)
		
		input, _ := os.ReadFile(destPath)
		os.WriteFile(finalPath, input, 0755)
		
		pm.logger.Printf("✓ Package downloaded to: %s", finalPath)
	}

	if !opts.KeepArchive {
		os.Remove(destPath)
	}

	return nil
}

func (pm *PackageManager) Search(ctx context.Context, query string) ([]PackageEntry, error) {
	return pm.client.Search(ctx, query)
}

func (pm *PackageManager) GetInfo(ctx context.Context, name string) (*Manifest, error) {
	// Try direct lookup first
	if strings.Contains(name, ".") {
		entry, err := pm.client.GetPackage(ctx, name)
		if err == nil {
			version := entry.Latest.Version
			if version == "" && len(entry.Versions) > 0 {
				version = entry.Versions[len(entry.Versions)-1]
			}
			if version == "" { version = "latest" }
			return pm.client.GetManifest(ctx, entry.ID, version)
		}
	}

	// Fallback to Search
	results, err := pm.client.Search(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("package not found")
	}
	
	entry := pm.selectBestMatch(results, name)
	
	version := entry.Latest.Version
	if version == "" && len(entry.Versions) > 0 {
		version = entry.Versions[len(entry.Versions)-1]
	}
	if version == "" { version = "latest" }

	return pm.client.GetManifest(ctx, entry.ID, version)
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
	// 3. Suffix Match (e.g. "wget" matches "JernejSimoncic.Wget")
	suffix := "." + strings.ToLower(query)
	for _, p := range results {
		if strings.HasSuffix(strings.ToLower(p.ID), suffix) {
			return &p
		}
	}
	// 4. Fallback
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
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}