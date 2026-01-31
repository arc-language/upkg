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
	
	if version == "" || version == "latest" {
		var entry *PackageEntry
		var err error

		// Strategy A: Direct ID Lookup (Preferred)
		// We try this first, but it is case-sensitive on the API side.
		if strings.Contains(opts.Package, ".") {
			pm.logger.Printf("Attempting direct ID lookup for '%s'...", opts.Package)
			entry, err = pm.client.GetPackage(ctx, opts.Package)
			if err == nil {
				pm.logger.Printf("✓ Found package via ID: %s", entry.ID)
			} else {
				pm.logger.Printf("Direct lookup failed (likely case mismatch or not found): %v", err)
			}
		}

		// Strategy B: Search (Fallback)
		// If direct lookup failed, we search. Search is fuzzy and case-insensitive.
		if entry == nil {
			pm.logger.Printf("Falling back to search for '%s'...", opts.Package)
			results, err := pm.client.Search(ctx, opts.Package)
			
			// If standard search fails and ID has dots, try replacing with spaces
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

			// Select best match
			entry = pm.selectBestMatch(results, opts.Package)
			pm.logger.Printf("Selected from search: %s", entry.ID)
		}

		id = entry.ID
		
		// Version Resolution
		versions := entry.GetVersions()
		
		if entry.Latest.Version != "" {
			version = entry.Latest.Version
		} else if len(versions) > 0 {
			// Take the last version in the list (usually the highest)
			version = versions[len(versions)-1]
		} else {
			// Critical failure point: No version found.
			// winget.run/v2/manifests/... requires a specific version. "latest" usually fails.
			return fmt.Errorf("unable to resolve version for %s: no versions returned by API", id)
		}
		pm.logger.Printf("Resolved Version: %s", version)
	}

	// Get Manifest
	manifest, err := pm.client.GetManifest(ctx, id, version)
	if err != nil {
		return fmt.Errorf("fetching manifest for %s @ %s: %w", id, version, err)
	}

	// Select Installer
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
		return fmt.Errorf("no compatible installer found for %s", wingetArch)
	}

	pm.logger.Printf("Selected installer: %s (%s)", bestInstaller.InstallerType, bestInstaller.Architecture)

	// Download
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

	// Verify Hash
	if opts.VerifyHash && bestInstaller.InstallerSha256 != "" {
		pm.logger.Printf("Verifying hash...")
		if err := verifyHash(destPath, bestInstaller.InstallerSha256); err != nil {
			return err
		}
	}

	// Install
	installDir := filepath.Join(pm.config.InstallPath, manifest.PackageIdentifier)
	isZip := bestInstaller.InstallerType == "zip" || strings.HasSuffix(strings.ToLower(bestInstaller.InstallerUrl), ".zip")
	
	if opts.Extract && isZip {
		pm.logger.Printf("Extracting to %s", installDir)
		if err := unzip(destPath, installDir); err != nil {
			return fmt.Errorf("unzip failed: %w", err)
		}
	} else {
		pm.logger.Printf("Installing to %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return err
		}
		finalPath := filepath.Join(installDir, fileName)
		input, _ := os.ReadFile(destPath)
		os.WriteFile(finalPath, input, 0755)
	}

	if !opts.KeepArchive {
		os.Remove(destPath)
	}

	return nil
}

// ... include helpers (selectBestMatch, etc.) from previous response ...
func (pm *PackageManager) selectBestMatch(results []PackageEntry, query string) *PackageEntry {
	for _, p := range results {
		if strings.EqualFold(p.ID, query) { return &p }
	}
	for _, p := range results {
		if strings.EqualFold(p.Latest.Name, query) { return &p }
	}
	suffix := "." + strings.ToLower(query)
	for _, p := range results {
		if strings.HasSuffix(strings.ToLower(p.ID), suffix) { return &p }
	}
	return &results[0]
}

// ... existing helpers (determineExtension, sanitizeFilename, verifyHash, unzip) ...
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
	return strings.ReplaceAll(name, "\\", "_")
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