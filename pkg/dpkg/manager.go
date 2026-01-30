// pkg/dpkg/manager.go
package dpkg

import (
	"archive/tar"
	"compress/gzip"
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

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// NewPackageManager creates a new Debian package manager
func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.RepositoryURL == "" {
		cfg.RepositoryURL = DefaultRepositoryURL
	}
	if cfg.SecurityURL == "" {
		cfg.SecurityURL = DefaultSecurityURL
	}
	if cfg.Release == "" {
		cfg.Release = DefaultRelease
	}
	if cfg.Component == "" {
		cfg.Component = DefaultComponent
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.CachePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "dpkg")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	// Setup logger
	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[DPKG] ", log.LstdFlags)
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
		pm.logger.Printf("Initialized Debian PackageManager")
		pm.logger.Printf("  Repository: %s", cfg.RepositoryURL)
		pm.logger.Printf("  Release: %s", cfg.Release)
		pm.logger.Printf("  Component: %s", cfg.Component)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  CachePath: %s", cfg.CachePath)
	}

	return pm
}

// Download downloads and installs a Debian package and its dependencies
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts == nil || opts.Package == "" {
		return fmt.Errorf("Package is required in DownloadOptions")
	}

	pm.logger.Printf("Starting installation for package: %s", opts.Package)

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

	// 1. Update package index once at the top level
	pm.logger.Printf("Updating package index...")
	if err := pm.updatePackageIndex(ctx, opts.Architecture); err != nil {
		return fmt.Errorf("updating package index: %w", err)
	}
	pm.logger.Printf("✓ Package index updated")

	// Track installed packages to avoid loops
	visited := make(map[string]bool)

	// Start recursive installation
	return pm.installRecursive(ctx, opts, visited)
}

// installRecursive handles the actual download, dependency resolution, and extraction
func (pm *PackageManager) installRecursive(ctx context.Context, opts *DownloadOptions, visited map[string]bool) error {
	// Skip if already visited in this transaction
	if visited[opts.Package] {
		return nil
	}
	visited[opts.Package] = true

	pm.logger.Printf("Processing package: %s", opts.Package)

	// 1. Find package info
	pkgInfo, err := pm.findPackage(opts.Package, opts.Version, opts.Architecture)
	if err != nil {
		return fmt.Errorf("finding package %s: %w", opts.Package, err)
	}

	// 2. Resolve and install dependencies FIRST
	if len(pkgInfo.Depends) > 0 {
		pm.logger.Printf("Resolving dependencies for %s...", opts.Package)
		for _, depName := range pkgInfo.Depends {
			pm.logger.Printf("  -> Dependency: %s", depName)
			
			depOpts := *opts // Shallow copy options
			depOpts.Package = depName
			depOpts.Version = "" // Use latest/default version for dependency
			
			if err := pm.installRecursive(ctx, &depOpts, visited); err != nil {
				// Log warning but proceed, as some deps might be virtual/optional/pre-installed
				pm.logger.Printf("  ⚠️  Warning: failed to install dependency %s: %v", depName, err)
			}
		}
	}

	// 3. Download package
	pm.logger.Printf("Downloading %s...", pkgInfo.Package)
	debPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s_%s_%s.deb", pkgInfo.Package, pkgInfo.Version, pkgInfo.Architecture))

	downloadURL := fmt.Sprintf("%s/%s", pm.config.RepositoryURL, pkgInfo.Filename)
	if err := pm.downloadPackage(ctx, downloadURL, debPath); err != nil {
		return fmt.Errorf("downloading package %s: %w", pkgInfo.Package, err)
	}

	// 4. Verify hash
	if opts.VerifyHash && pkgInfo.SHA256 != "" {
		if err := pm.verifyFileHash(debPath, pkgInfo.SHA256); err != nil {
			return fmt.Errorf("hash verification failed for %s: %w", pkgInfo.Package, err)
		}
	}

	// 5. Extract package
	if opts.Extract {
		pm.logger.Printf("Extracting %s...", pkgInfo.Package)
		if err := pm.extractDebPackage(debPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting package %s: %w", pkgInfo.Package, err)
		}

		// 6. Cleanup archive if requested
		if !opts.KeepArchive {
			os.Remove(debPath)
		}
	}

	pm.logger.Printf("✓ Installed %s", pkgInfo.Package)
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

	// Common Debian components to search
	components := []string{"main", "contrib", "non-free", "non-free-firmware"}
	
	totalPackages := 0
	for _, component := range components {
		// Construct URL for Packages.gz
		url := fmt.Sprintf("%s/dists/%s/%s/binary-%s/Packages.gz",
			pm.config.RepositoryURL,
			pm.config.Release,
			component,
			arch)

		pm.logger.Printf("  Fetching %s component: %s", component, url)

		// Download and decompress
		reader, err := pm.client.GetGzipped(ctx, url)
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to fetch %s component: %v", component, err)
			continue
		}

		// Parse packages
		packages, err := ParsePackages(reader)
		reader.Close()
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to parse %s component: %v", component, err)
			continue
		}

		// Add to cache
		for _, pkg := range packages {
			// Create a unique key including the component is not strictly necessary for lookups
			// but helps if we want to debug
			key := fmt.Sprintf("%s_%s", pkg.Package, pkg.Architecture)
			pm.cache.packages[key] = pkg
		}
		
		totalPackages += len(packages)
	}

	pm.logger.Printf("  Total packages indexed: %d", totalPackages)
	pm.cache.lastUpdate = time.Now()

	return nil
}

// findPackage finds a package in the cache
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	// Try exact architecture first
	key := fmt.Sprintf("%s_%s", name, arch)
	if pkg, ok := pm.cache.packages[key]; ok {
		if version == "" || pkg.Version == version {
			return pkg, nil
		}
	}

	// Try "all" architecture
	key = fmt.Sprintf("%s_%s", name, ArchAll)
	if pkg, ok := pm.cache.packages[key]; ok {
		if version == "" || pkg.Version == version {
			return pkg, nil
		}
	}

	if version != "" {
		return nil, fmt.Errorf("package %s version %s not found", name, version)
	}
	return nil, fmt.Errorf("package %s not found", name)
}

// downloadPackage downloads a .deb package
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
		return fmt.Errorf("downloading: %w", err)
	}

	return nil
}

// verifyFileHash verifies the SHA256 hash of a file
func (pm *PackageManager) verifyFileHash(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

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

// extractDebPackage extracts a .deb package using the ar and tar formats
func (pm *PackageManager) extractDebPackage(debPath, installPath string) error {
	// Open the .deb file (which is an ar archive)
	f, err := os.Open(debPath)
	if err != nil {
		return fmt.Errorf("opening .deb file: %w", err)
	}
	defer f.Close()

	// Create ar reader
	arReader := ar.NewReader(f)

	// Extract data.tar.* (contains the actual files)
	for {
		header, err := arReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading ar entry: %w", err)
		}

		// Look for data.tar.* (data.tar.xz, data.tar.gz, data.tar.zst, etc.)
		if strings.HasPrefix(header.Name, "data.tar") {
			return pm.extractDataTar(arReader, header.Name, installPath)
		}
	}

	return fmt.Errorf("no data.tar.* found in .deb package")
}

// extractDataTar extracts the data.tar.* from a .deb package
func (pm *PackageManager) extractDataTar(r io.Reader, name, installPath string) error {
	var tarReader *tar.Reader

	// Handle different compression formats
	if strings.HasSuffix(name, ".gz") {
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else if strings.HasSuffix(name, ".xz") {
		xzReader, err := xz.NewReader(r)
		if err != nil {
			return fmt.Errorf("creating xz reader: %w", err)
		}
		tarReader = tar.NewReader(xzReader)
	} else if strings.HasSuffix(name, ".zst") {
		zstdReader, err := zstd.NewReader(r)
		if err != nil {
			return fmt.Errorf("creating zstd reader: %w", err)
		}
		defer zstdReader.Close()
		tarReader = tar.NewReader(zstdReader)
	} else {
		// Assume uncompressed tar
		tarReader = tar.NewReader(r)
	}

	// Extract each entry
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Clean the path (remove leading ./)
		cleanPath := strings.TrimPrefix(header.Name, "./")
		if cleanPath == "" || cleanPath == "." {
			continue
		}

		targetPath := filepath.Join(installPath, cleanPath)

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, err)
			}

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory for symlink: %w", err)
			}
			// Remove existing symlink if it exists
			os.Remove(targetPath)
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", targetPath, header.Linkname, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("creating file %s: %w", targetPath, err)
			}

			written, err := io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("writing file %s: %w", targetPath, err)
			}

			if written != header.Size {
				return fmt.Errorf("file size mismatch for %s: expected %d, got %d", targetPath, header.Size, written)
			}
		}
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
		if strings.Contains(strings.ToLower(pkg.Package), query) ||
			strings.Contains(strings.ToLower(pkg.Description), query) {
			results = append(results, pkg)
		}
	}

	return results, nil
}