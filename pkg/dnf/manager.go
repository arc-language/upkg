// pkg/dnf/manager.go
package dnf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sassoftware/go-rpmutils"
)

// Dependency type classification
type depType int

const (
	depTypePackage depType = iota // Regular package name
	depTypeFile                    // File path dependency
	depTypeSoname                  // Shared library soname
	depTypeSymbol                  // Versioned symbol
	depTypeVirtual                 // Virtual capability
)

// classifyDependency determines what type of dependency this is
func classifyDependency(dep string) depType {
	// File path
	if strings.HasPrefix(dep, "/") {
		return depTypeFile
	}
	
	// Soname or versioned symbol (contains .so)
	if strings.Contains(dep, ".so") {
		return depTypeSoname
	}
	
	// Virtual capabilities with parentheses but no .so
	if strings.Contains(dep, "(") {
		return depTypeVirtual
	}
	
	// Regular package name
	return depTypePackage
}

// cleanDependencyName extracts the base name from a dependency
// e.g., "package >= 1.2.3" -> "package"
func cleanDependencyName(dep string) string {
	// Remove version operators and constraints
	for _, op := range []string{">=", "<=", "=", ">", "<"} {
		if idx := strings.Index(dep, op); idx != -1 {
			return strings.TrimSpace(dep[:idx])
		}
	}
	return strings.TrimSpace(dep)
}

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
	// 1. Resolve Package (might be a package name or a soname/capability)
	pkgInfo, err := pm.resolvePackage(pkgRequest, arch)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", pkgRequest, err)
	}

	// Skip if already processed
	if visited[pkgInfo.Name] {
		return nil
	}
	visited[pkgInfo.Name] = true

	pm.logger.Printf("Processing package: %s", pkgInfo.Name)

	// 2. Process Dependencies
	for _, req := range pkgInfo.Requires {
		// Skip self-reference
		if cleanDependencyName(req) == pkgInfo.Name {
			continue
		}
		
		// Try to resolve the dependency
		pm.logger.Printf("  -> Dependency: %s", req)
		
		if err := pm.installRecursive(ctx, req, arch, visited, opts); err != nil {
			// Check if this is a file dependency - those are often pre-satisfied
			if classifyDependency(req) == depTypeFile {
				if pm.config.Debug {
					pm.logger.Printf("    (file dependency, likely pre-satisfied)")
				}
				continue
			}
			
			// Warn but continue - dependency might be optional or runtime-only
			pm.logger.Printf("  ⚠️  Warning: %v", err)
		}
	}

	// 3. Download package
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

	// Download if not cached
	if _, err := os.Stat(rpmPath); os.IsNotExist(err) {
		pm.logger.Printf("Downloading %s...", pkgInfo.Name)
		if err := pm.downloadPackage(ctx, downloadURL, rpmPath); err != nil {
			return fmt.Errorf("downloading package: %w", err)
		}
	} else {
		pm.logger.Printf("  Using cached: %s", rpmPath)
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

// resolvePackage finds a package by name or by what it provides
// Handles package names, sonames, and other capabilities
func (pm *PackageManager) resolvePackage(name string, arch Architecture) (*PackageInfo, error) {
	clean := cleanDependencyName(name)

	// Try direct package name lookup first (most common case)
	if pkg, ok := pm.cache.packages[clean]; ok {
		return pkg, nil
	}

	// Try providers map (handles sonames, virtual packages, etc.)
	if providers, ok := pm.cache.providers[clean]; ok && len(providers) > 0 {
		// Prefer exact architecture match
		for _, p := range providers {
			if p.Architecture == string(arch) {
				return p, nil
			}
		}
		// Then noarch
		for _, p := range providers {
			if p.Architecture == "noarch" {
				return p, nil
			}
		}
		// Fallback to first available
		return providers[0], nil
	}

	return nil, fmt.Errorf("package or capability '%s' not found", clean)
}

// updatePackageIndex updates the local package index cache
func (pm *PackageManager) updatePackageIndex(ctx context.Context, arch Architecture) error {
	if time.Since(pm.cache.lastUpdate) < pm.cache.cacheDuration && len(pm.cache.packages) > 0 {
		return nil
	}

	pm.logger.Printf("Fetching package index from repository...")
	pm.cache.packages = make(map[string]*PackageInfo)
	pm.cache.providers = make(map[string][]*PackageInfo)

	// Construct URL
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

	resp, err := pm.client.Get(ctx, repomdURL)
	if err != nil {
		return fmt.Errorf("fetching repomd.xml: %w", err)
	}

	repoMD, err := ParseRepoMD(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("parsing repomd.xml: %w", err)
	}

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

	packages, err := ParsePrimary(primaryReader)
	if err != nil {
		return fmt.Errorf("parsing primary.xml: %w", err)
	}

	// Build package and provider indexes
	packageCount := 0
	
	for _, pkg := range packages {
		// Only index packages for the target architecture or noarch
		if pkg.Architecture != string(arch) && pkg.Architecture != "noarch" {
			continue
		}
		packageCount++

		// Index by package name
		pm.cache.packages[pkg.Name] = pkg
		
		// Package name always provides itself
		pm.cache.providers[pkg.Name] = append(pm.cache.providers[pkg.Name], pkg)
		
		// Index ALL provides (including sonames, virtual capabilities, etc.)
		// This is how DNF resolves soname dependencies like libssl.so.3()(64bit) -> openssl-libs
		for _, provide := range pkg.Provides {
			pm.cache.providers[provide] = append(pm.cache.providers[provide], pkg)
		}
	}

	pm.logger.Printf("  ✓ Indexed %d packages, %d unique provides", packageCount, len(pm.cache.providers))
	pm.cache.lastUpdate = time.Now()

	return nil
}

// findPackage is exposed for the generic Manager interface
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	return pm.resolvePackage(name, arch)
}

// downloadPackage downloads an .rpm package
func (pm *PackageManager) downloadPackage(ctx context.Context, url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	_, err = pm.client.Download(ctx, url, f)
	if err != nil {
		os.Remove(destPath)
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

// extractRPMPackage with Retry on Permission Denied
func (pm *PackageManager) extractRPMPackage(rpmPath, installPath string) error {
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	// Function to perform extraction
	extract := func() error {
		f, err := os.Open(rpmPath)
		if err != nil {
			return err
		}
		defer f.Close()

		rpm, err := rpmutils.ReadRpm(f)
		if err != nil {
			return err
		}

		if err := rpm.ExpandPayload(installPath); err != nil {
			return err
		}
		return nil
	}

	// Try extraction
	err := extract()
	
	// If permission error, fix permissions and retry
	if err != nil && (strings.Contains(err.Error(), "permission denied") || strings.Contains(err.Error(), "access denied")) {
		pm.logger.Printf("  ⚠️  Permission denied during extraction. Fixing permissions and retrying...")
		
		// Force write permissions on everything in install path
		errFix := filepath.WalkDir(installPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil { return nil } // Ignore walk errors
			
			info, err := d.Info()
			if err != nil { return nil }
			
			mode := info.Mode()
			newMode := mode | 0200 // Add user write permission
			if mode.IsDir() {
				newMode |= 0700 // Add user read/write/execute for dirs
			}
			
			if mode != newMode {
				_ = os.Chmod(path, newMode)
			}
			return nil
		})
		
		if errFix != nil {
			pm.logger.Printf("  ⚠️  Failed to fix permissions: %v", errFix)
		}

		// Retry extraction
		err = extract()
	}

	if err != nil {
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
		if strings.Contains(strings.ToLower(pkg.Name), query) {
			results = append(results, pkg)
		}
	}

	return results, nil
}