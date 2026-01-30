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
	"path/filepath"
	"strings"
	"time"

	"github.com/sassoftware/go-rpmutils"
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
			providers:     make(map[string][]*PackageInfo),
			cacheDuration: 30 * time.Minute,
		},
	}

	if cfg.Debug {
		pm.logger.Printf("Initialized Fedora DNF PackageManager")
		pm.logger.Printf("  Release: %s", cfg.Release)
	}

	return pm
}

// Download downloads and installs a Fedora package
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts == nil || opts.Package == "" {
		return fmt.Errorf("Package is required in DownloadOptions")
	}

	pm.logger.Printf("Starting operation for package: %s", opts.Package)

	// Set defaults
	if opts.Architecture == "" {
		detected, err := DetectArchitecture()
		if err != nil {
			return fmt.Errorf("detecting architecture: %w", err)
		}
		opts.Architecture = detected
	}

	// 1. Update package index (Global sync)
	if err := pm.updatePackageIndex(ctx, opts.Architecture); err != nil {
		return fmt.Errorf("updating package index: %w", err)
	}

	// Track visited to avoid cycles
	visited := make(map[string]bool)

	// Start recursion
	return pm.installRecursive(ctx, opts.Package, opts.Architecture, visited, opts)
}

// installRecursive resolves dependencies and installs the package
func (pm *PackageManager) installRecursive(ctx context.Context, pkgRequest string, arch Architecture, visited map[string]bool, opts *DownloadOptions) error {
	// 1. Resolve Package (handle name or virtual provide like /bin/sh)
	pkgInfo, err := pm.resolvePackage(pkgRequest)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", pkgRequest, err)
	}

	// Avoid cycles using the unique package name
	if visited[pkgInfo.Name] {
		return nil
	}
	visited[pkgInfo.Name] = true

	pm.logger.Printf("Processing package: %s (resolved from %s)", pkgInfo.Name, pkgRequest)

	// 2. Install Dependencies
	for _, req := range pkgInfo.Requires {
		// Filter out internal RPM provides that aren't packages
		if strings.HasPrefix(req, "rpmlib(") { continue }
		if strings.HasPrefix(req, "config(") { continue }
		
		// Clean the requirement string (remove version logic for now)
		// e.g., "glibc >= 2.34" -> "glibc"
		reqName := cleanReqName(req)
		
		// Skip self-reference
		if reqName == pkgInfo.Name { continue }

		pm.logger.Printf("  -> Dependency: %s", reqName)
		
		if err := pm.installRecursive(ctx, reqName, arch, visited, opts); err != nil {
			// DNF metadata is very detailed. Missing one specific capability (like a specific .so version)
			// shouldn't halt the whole install in this lightweight manager.
			pm.logger.Printf("  ⚠️  Warning: failed to install dependency %s: %v", reqName, err)
		}
	}

	// 3. Download
	rpmPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s-%s.%s.rpm", pkgInfo.Name, pkgInfo.FullVersion(), pkgInfo.Architecture))

	// Construct download URL
	baseURL := pm.config.RepositoryURL
	var downloadURL string

	if pm.config.Repository == "updates" {
		downloadURL = fmt.Sprintf("%s/updates/%s/Everything/%s/%s",
			baseURL, pm.config.Release, arch, pkgInfo.Location)
	} else if pm.config.Release == "rawhide" {
		downloadURL = fmt.Sprintf("%s/development/rawhide/Everything/%s/os/%s",
			baseURL, arch, pkgInfo.Location)
	} else {
		downloadURL = fmt.Sprintf("%s/releases/%s/Everything/%s/os/%s",
			baseURL, pm.config.Release, arch, pkgInfo.Location)
	}

	pm.logger.Printf("Downloading %s...", pkgInfo.Name)
	if err := pm.downloadPackage(ctx, downloadURL, rpmPath); err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}

	// 4. Verify hash
	if opts.VerifyHash && pkgInfo.Checksum != "" {
		if err := pm.verifyFileHash(rpmPath, pkgInfo.Checksum, pkgInfo.ChecksumType); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// 5. Extract package
	if opts.Extract {
		pm.logger.Printf("Extracting %s...", pkgInfo.Name)
		if err := pm.extractRPMPackage(rpmPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting package: %w", err)
		}
		
		if !opts.KeepArchive {
			os.Remove(rpmPath)
		}
	}

	pm.logger.Printf("✓ Installed %s", pkgInfo.Name)
	return nil
}

// resolvePackage finds a package by name OR by what it provides
func (pm *PackageManager) resolvePackage(name string) (*PackageInfo, error) {
	// 1. Try direct name match
	if pkg, ok := pm.cache.packages[name]; ok {
		return pkg, nil
	}

	// 2. Try virtual providers (e.g. "libssl.so.3" or "/bin/sh")
	if providers, ok := pm.cache.providers[name]; ok && len(providers) > 0 {
		// Heuristic: Prefer package with shortest name (e.g. "bash" over "bash-doc")
		// or just take the first one.
		return providers[0], nil
	}

	return nil, fmt.Errorf("package or provider '%s' not found", name)
}

func cleanReqName(req string) string {
	// Basic cleaning: remove version comparisons
	// "bash >= 5.0" -> "bash"
	if idx := strings.IndexAny(req, "<>="); idx != -1 {
		return strings.TrimSpace(req[:idx])
	}
	return req
}

// updatePackageIndex updates the local package index cache
func (pm *PackageManager) updatePackageIndex(ctx context.Context, arch Architecture) error {
	// Check if cache is still valid
	if time.Since(pm.cache.lastUpdate) < pm.cache.cacheDuration && len(pm.cache.packages) > 0 {
		return nil
	}

	pm.logger.Printf("Fetching package index from repository...")
	pm.cache.packages = make(map[string]*PackageInfo)
	pm.cache.providers = make(map[string][]*PackageInfo)

	// Construct base URL and repomd.xml URL
	baseURL := pm.config.RepositoryURL
	var repomdURL string

	if pm.config.Repository == "updates" {
		repomdURL = fmt.Sprintf("%s/updates/%s/Everything/%s/repodata/repomd.xml",
			baseURL, pm.config.Release, arch)
	} else if pm.config.Release == "rawhide" {
		repomdURL = fmt.Sprintf("%s/development/rawhide/Everything/%s/os/repodata/repomd.xml",
			baseURL, arch)
	} else {
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

	pm.logger.Printf("  Downloading primary metadata...")

	// Download and decompress primary.xml
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

	// Index packages and providers
	for _, pkg := range packages {
		// 1. Map Name -> Package
		pm.cache.packages[pkg.Name] = pkg
		
		// 2. Map Provides -> Package
		// Also treat the package name itself as a provider
		pm.cache.providers[pkg.Name] = append(pm.cache.providers[pkg.Name], pkg)
		
		for _, provide := range pkg.Provides {
			pm.cache.providers[provide] = append(pm.cache.providers[provide], pkg)
		}
	}

	pm.logger.Printf("  ✓ Parsed %d packages, %d unique providers", len(packages), len(pm.cache.providers))
	pm.cache.lastUpdate = time.Now()

	return nil
}

// findPackage is exposed for the generic Manager interface
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	return pm.resolvePackage(name)
}

// downloadPackage downloads an .rpm package
func (pm *PackageManager) downloadPackage(ctx context.Context, url, destPath string) error {
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
	_, err = pm.client.Download(ctx, url, f)
	if err != nil {
		os.Remove(destPath) // clean up partial
		return fmt.Errorf("downloading: %w", err)
	}

	return nil
}

// verifyFileHash verifies the checksum of a file
func (pm *PackageManager) verifyFileHash(filePath, expectedHash, hashType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// For now, we only support SHA256 as it's standard in Fedora
	if hashType != "sha256" {
		return nil
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("computing hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// extractRPMPackage extracts an .rpm package
func (pm *PackageManager) extractRPMPackage(rpmPath, installPath string) error {
	// Ensure install directory exists
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	// Open the RPM file
	f, err := os.Open(rpmPath)
	if err != nil {
		return fmt.Errorf("opening rpm file: %w", err)
	}
	defer f.Close()

	// Read the RPM package
	rpm, err := rpmutils.ReadRpm(f)
	if err != nil {
		return fmt.Errorf("reading rpm package: %w", err)
	}

	// Extract the payload to the install path
	if err := rpm.ExpandPayload(installPath); err != nil {
		return fmt.Errorf("expanding rpm payload: %w", err)
	}

	return nil
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
			strings.Contains(strings.ToLower(pkg.Summary), query) {
			results = append(results, pkg)
		}
	}

	return results, nil
}