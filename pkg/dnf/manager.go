// pkg/dnf/manager.go
package dnf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NewPackageManager creates a new Fedora/DNF package manager
func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.RepositoryURL == "" {
		cfg.RepositoryURL = DefaultRepositoryURL
	}
	if cfg.Release == "" {
		cfg.Release = DefaultRelease
	}
	if cfg.Repository == "" {
		cfg.Repository = DefaultRepository
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.CachePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "dnf")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	// Setup logger
	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[DNF] ", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}
	}

	pm := &PackageManager{
		client: NewClientWithTimeout(cfg.Timeout),
		config: cfg,
		logger: logger,
		cache: &PackageCache{
			packages:      make(map[string]*PackageInfo),
			cacheDuration: 30 * time.Minute,
		},
	}

	if cfg.Debug {
		pm.logger.Printf("Initialized Fedora DNF PackageManager")
		pm.logger.Printf("  Repository: %s", cfg.RepositoryURL)
		pm.logger.Printf("  Release: %s", cfg.Release)
		pm.logger.Printf("  Repository: %s", cfg.Repository)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  CachePath: %s", cfg.CachePath)
	}

	return pm
}

// Download downloads and installs a Fedora package
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts == nil || opts.Package == "" {
		return fmt.Errorf("Package is required in DownloadOptions")
	}

	pm.logger.Printf("Starting download for package: %s", opts.Package)

	// Set defaults
	if opts.Architecture == "" {
		detected, err := DetectArchitecture()
		if err != nil {
			return fmt.Errorf("detecting architecture: %w", err)
		}
		opts.Architecture = detected
		pm.logger.Printf("Auto-detected architecture: %s", opts.Architecture)
	} else {
		if !opts.Architecture.IsValid() {
			return fmt.Errorf("invalid architecture: %s", opts.Architecture)
		}
	}

	pm.logger.Printf("Download options:")
	pm.logger.Printf("  Package: %s", opts.Package)
	pm.logger.Printf("  Version: %s", opts.Version)
	pm.logger.Printf("  Architecture: %s", opts.Architecture)
	pm.logger.Printf("  Extract: %v", opts.Extract)
	pm.logger.Printf("  KeepArchive: %v", opts.KeepArchive)
	pm.logger.Printf("  VerifyHash: %v", opts.VerifyHash)

	// 1. Update package index if needed
	pm.logger.Printf("Step 1: Updating package index...")
	if err := pm.updatePackageIndex(ctx, opts.Architecture); err != nil {
		return fmt.Errorf("updating package index: %w", err)
	}
	pm.logger.Printf("  ✓ Package index updated")

	// 2. Find package info
	pm.logger.Printf("Step 2: Finding package info...")
	pkgInfo, err := pm.findPackage(opts.Package, opts.Version, opts.Architecture)
	if err != nil {
		return fmt.Errorf("finding package: %w", err)
	}
	pm.logger.Printf("  ✓ Package found: %s %s", pkgInfo.Name, pkgInfo.FullVersion())
	pm.logger.Printf("    Summary: %s", pkgInfo.Summary)
	pm.logger.Printf("    Size: %d bytes", pkgInfo.Size)

	// 3. Download package
	pm.logger.Printf("Step 3: Downloading package...")
	rpmPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s-%s.%s.rpm", pkgInfo.Name, pkgInfo.FullVersion(), pkgInfo.Architecture))

	// Construct download URL
	baseURL := pm.config.RepositoryURL
	var downloadURL string

	if pm.config.Repository == "updates" {
		downloadURL = fmt.Sprintf("%s/updates/%s/Everything/%s/%s",
			baseURL, pm.config.Release, opts.Architecture, pkgInfo.Location)
	} else if pm.config.Release == "rawhide" {
		downloadURL = fmt.Sprintf("%s/development/rawhide/Everything/%s/os/%s",
			baseURL, opts.Architecture, pkgInfo.Location)
	} else {
		downloadURL = fmt.Sprintf("%s/releases/%s/Everything/%s/os/%s",
			baseURL, pm.config.Release, opts.Architecture, pkgInfo.Location)
	}

	if err := pm.downloadPackage(ctx, downloadURL, rpmPath); err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}
	pm.logger.Printf("  ✓ Download complete")

	// 4. Verify hash
	if opts.VerifyHash && pkgInfo.Checksum != "" {
		pm.logger.Printf("Step 4: Verifying checksum...")
		if err := pm.verifyFileHash(rpmPath, pkgInfo.Checksum, pkgInfo.ChecksumType); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		pm.logger.Printf("  ✓ Checksum verified")
	} else {
		pm.logger.Printf("Step 4: Skipping checksum verification")
	}

	// 5. Extract package
	if opts.Extract {
		pm.logger.Printf("Step 5: Extracting package...")
		if err := pm.extractRPMPackage(rpmPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting package: %w", err)
		}
		pm.logger.Printf("  ✓ Extraction complete")

		// 6. Cleanup archive if requested
		if !opts.KeepArchive {
			pm.logger.Printf("Step 6: Removing archive file...")
			if err := os.Remove(rpmPath); err != nil {
				pm.logger.Printf("  ⚠️  Warning: failed to remove archive: %v", err)
			} else {
				pm.logger.Printf("  ✓ Archive removed")
			}
		} else {
			pm.logger.Printf("Step 6: Keeping archive file as requested")
		}
	} else {
		pm.logger.Printf("Step 5: Skipping extraction (Extract=false)")
	}

	pm.logger.Printf("✓ Package %s installed successfully", opts.Package)
	return nil
}

// updatePackageIndex updates the local package index cache
func (pm *PackageManager) updatePackageIndex(ctx context.Context, arch Architecture) error {
	// Check if cache is still valid
	if time.Since(pm.cache.lastUpdate) < pm.cache.cacheDuration && len(pm.cache.packages) > 0 {
		pm.logger.Printf("Using cached package index (age: %v)", time.Since(pm.cache.lastUpdate))
		return nil
	}

	pm.logger.Printf("Fetching package index from repository...")

	// Clear cache before updating
	pm.cache.packages = make(map[string]*PackageInfo)

	// Construct base URL and repomd.xml URL
	baseURL := pm.config.RepositoryURL
	var repomdURL string

	if pm.config.Repository == "updates" {
		// Updates repository structure
		repomdURL = fmt.Sprintf("%s/updates/%s/Everything/%s/repodata/repomd.xml",
			baseURL, pm.config.Release, arch)
	} else if pm.config.Release == "rawhide" {
		// Rawhide (development) structure
		repomdURL = fmt.Sprintf("%s/development/rawhide/Everything/%s/os/repodata/repomd.xml",
			baseURL, arch)
	} else {
		// Regular releases structure
		repomdURL = fmt.Sprintf("%s/releases/%s/Everything/%s/os/repodata/repomd.xml",
			baseURL, pm.config.Release, arch)
	}

	pm.logger.Printf("  Fetching repomd.xml: %s", repomdURL)

	// Download repomd.xml
	resp, err := pm.client.Get(ctx, repomdURL)
	if err != nil {
		return fmt.Errorf("fetching repomd.xml: %w", err)
	}

	// Parse repomd.xml
	repoMD, err := ParseRepoMD(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("parsing repomd.xml: %w", err)
	}

	// Find primary.xml location
	var primaryLocation string
	for _, data := range repoMD.Data {
		if data.Type == "primary" {
			primaryLocation = data.Location
			break
		}
	}

	if primaryLocation == "" {
		return fmt.Errorf("primary.xml location not found in repomd.xml")
	}

	pm.logger.Printf("  Found primary.xml: %s", primaryLocation)

	// Construct full URL for primary.xml
	var primaryURL string
	if pm.config.Repository == "updates" {
		primaryURL = fmt.Sprintf("%s/updates/%s/Everything/%s/%s",
			baseURL, pm.config.Release, arch, primaryLocation)
	} else if pm.config.Release == "rawhide" {
		primaryURL = fmt.Sprintf("%s/development/rawhide/Everything/%s/os/%s",
			baseURL, arch, primaryLocation)
	} else {
		primaryURL = fmt.Sprintf("%s/releases/%s/Everything/%s/os/%s",
			baseURL, pm.config.Release, arch, primaryLocation)
	}

	pm.logger.Printf("  Downloading primary.xml from: %s", primaryURL)

	// Download and decompress primary.xml (usually .gz, .xz, or .zst)
	var primaryReader io.ReadCloser
	if strings.HasSuffix(primaryLocation, ".gz") {
		primaryReader, err = pm.client.GetGzipped(ctx, primaryURL)
	} else if strings.HasSuffix(primaryLocation, ".xz") {
		primaryReader, err = pm.client.GetXZ(ctx, primaryURL)
	} else if strings.HasSuffix(primaryLocation, ".zst") {
		primaryReader, err = pm.client.GetZstd(ctx, primaryURL)
	} else {
		resp, err = pm.client.Get(ctx, primaryURL)
		if err == nil {
			primaryReader = resp.Body
		}
	}

	if err != nil {
		return fmt.Errorf("fetching primary.xml: %w", err)
	}
	defer primaryReader.Close()

	// Parse primary.xml
	packages, err := ParsePrimary(primaryReader)
	if err != nil {
		return fmt.Errorf("parsing primary.xml: %w", err)
	}

	pm.logger.Printf("  ✓ Parsed %d packages", len(packages))

	// Add to cache
	for _, pkg := range packages {
		key := fmt.Sprintf("%s_%s", pkg.Name, pkg.Architecture)
		pm.cache.packages[key] = pkg
	}

	pm.cache.lastUpdate = time.Now()

	return nil
}

// findPackage finds a package in the cache
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	// Try exact architecture first
	key := fmt.Sprintf("%s_%s", name, arch)
	if pkg, ok := pm.cache.packages[key]; ok {
		if version == "" || pkg.FullVersion() == version {
			return pkg, nil
		}
	}

	// Try "noarch" architecture
	key = fmt.Sprintf("%s_%s", name, ArchNoarch)
	if pkg, ok := pm.cache.packages[key]; ok {
		if version == "" || pkg.FullVersion() == version {
			return pkg, nil
		}
	}

	if version != "" {
		return nil, fmt.Errorf("package %s version %s not found for architecture %s", name, version, arch)
	}
	return nil, fmt.Errorf("package %s not found for architecture %s", name, arch)
}

// downloadPackage downloads an .rpm package
func (pm *PackageManager) downloadPackage(ctx context.Context, url, destPath string) error {
	pm.logger.Printf("Downloading from: %s", url)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Create output file
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	// Download
	written, err := pm.client.Download(ctx, url, f)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}

	pm.logger.Printf("  Downloaded %d bytes to %s", written, destPath)
	return nil
}

// verifyFileHash verifies the checksum of a file
func (pm *PackageManager) verifyFileHash(filePath, expectedHash, hashType string) error {
	pm.logger.Printf("Computing %s checksum of: %s", hashType, filePath)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// For now, we only support SHA256
	if hashType != "sha256" {
		pm.logger.Printf("  Skipping verification: unsupported hash type %s", hashType)
		return nil
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("computing hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	pm.logger.Printf("  Expected: %s", expectedHash)
	pm.logger.Printf("  Actual:   %s", actualHash)

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	pm.logger.Printf("  ✓ Hashes match!")
	return nil
}

// extractRPMPackage extracts an .rpm package using rpm2cpio and cpio
func (pm *PackageManager) extractRPMPackage(rpmPath, installPath string) error {
	pm.logger.Printf("Extracting .rpm package: %s -> %s", rpmPath, installPath)

	// Ensure install directory exists
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	// Check if rpm2cpio is available
	if _, err := exec.LookPath("rpm2cpio"); err != nil {
		pm.logger.Printf("  rpm2cpio not found, attempting to extract with Go implementation")
		return pm.extractRPMNative(rpmPath, installPath)
	}

	// Use rpm2cpio | cpio to extract
	pm.logger.Printf("  Using rpm2cpio and cpio for extraction")

	// rpm2cpio converts rpm to cpio format
	rpm2cpioCmd := exec.Command("rpm2cpio", rpmPath)
	cpioCmd := exec.Command("cpio", "-idmv")
	cpioCmd.Dir = installPath

	// Pipe rpm2cpio output to cpio input
	pipe, err := rpm2cpioCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating pipe: %w", err)
	}
	cpioCmd.Stdin = pipe

	// Start both commands
	if err := rpm2cpioCmd.Start(); err != nil {
		return fmt.Errorf("starting rpm2cpio: %w", err)
	}
	if err := cpioCmd.Start(); err != nil {
		return fmt.Errorf("starting cpio: %w", err)
	}

	// Wait for both to complete
	if err := rpm2cpioCmd.Wait(); err != nil {
		return fmt.Errorf("rpm2cpio failed: %w", err)
	}
	if err := cpioCmd.Wait(); err != nil {
		return fmt.Errorf("cpio failed: %w", err)
	}

	pm.logger.Printf("  ✓ Extraction complete")
	return nil
}

// extractRPMNative is a fallback native Go implementation for RPM extraction
func (pm *PackageManager) extractRPMNative(rpmPath, installPath string) error {
	pm.logger.Printf("  Note: Native RPM extraction is limited. Install rpm2cpio for best results.")
	pm.logger.Printf("  Run: sudo dnf install rpm (Fedora) or sudo apt install rpm2cpio (Debian/Ubuntu)")
	return fmt.Errorf("rpm2cpio not available - please install it: sudo dnf install rpm")
}

// GetPackageInfo retrieves information about a package
func (pm *PackageManager) GetPackageInfo(ctx context.Context, name string, arch Architecture) (*PackageInfo, error) {
	if arch == "" {
		var err error
		arch, err = DetectArchitecture()
		if err != nil {
			return nil, err
		}
	}

	if err := pm.updatePackageIndex(ctx, arch); err != nil {
		return nil, err
	}

	return pm.findPackage(name, "", arch)
}

// SearchPackages searches for packages by name
func (pm *PackageManager) SearchPackages(ctx context.Context, query string, arch Architecture) ([]*PackageInfo, error) {
	if arch == "" {
		var err error
		arch, err = DetectArchitecture()
		if err != nil {
			return nil, err
		}
	}

	if err := pm.updatePackageIndex(ctx, arch); err != nil {
		return nil, err
	}

	var results []*PackageInfo
	query = strings.ToLower(query)

	for _, pkg := range pm.cache.packages {
		if strings.Contains(strings.ToLower(pkg.Name), query) ||
			strings.Contains(strings.ToLower(pkg.Summary), query) ||
			strings.Contains(strings.ToLower(pkg.Description), query) {
			results = append(results, pkg)
		}
	}

	return results, nil
}