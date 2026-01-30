// pkg/apk/manager.go
package apk

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewPackageManager creates a new Alpine package manager
func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.RepositoryURL == "" {
		cfg.RepositoryURL = DefaultRepositoryURL
	}
	if cfg.Branch == "" {
		cfg.Branch = DefaultBranch
	}
	if cfg.Repository == "" {
		cfg.Repository = DefaultRepository
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.CachePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "apk")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	// Setup logger
	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[APK] ", log.LstdFlags)
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
		pm.logger.Printf("Initialized Alpine APK PackageManager")
		pm.logger.Printf("  Repository: %s", cfg.RepositoryURL)
		pm.logger.Printf("  Branch: %s", cfg.Branch)
		pm.logger.Printf("  Repository: %s", cfg.Repository)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  CachePath: %s", cfg.CachePath)
	}

	return pm
}

// Download downloads and installs an Alpine package and its dependencies
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
	// Skip if already visited
	if visited[opts.Package] {
		return nil
	}
	visited[opts.Package] = true

	pm.logger.Printf("Processing package: %s", opts.Package)

	// 1. Find package info
	pkgInfo, err := pm.findPackage(opts.Package, opts.Version, opts.Architecture)
	if err != nil {
		// Alpine dependencies often use virtual names like "so:libc.so.6" or "cmd:sh"
		// Since we don't have a "provides" DB yet, we skip these cleanly to avoid crashing.
		if strings.HasPrefix(opts.Package, "so:") || strings.HasPrefix(opts.Package, "cmd:") {
			pm.logger.Printf("  ℹ️  Skipping virtual dependency: %s", opts.Package)
			return nil
		}
		return fmt.Errorf("finding package %s: %w", opts.Package, err)
	}

	// 2. Resolve and install dependencies FIRST
	if len(pkgInfo.Depends) > 0 {
		pm.logger.Printf("Resolving dependencies for %s...", opts.Package)
		for _, depName := range pkgInfo.Depends {
			// Skip self-references
			if depName == opts.Package {
				continue
			}
			
			pm.logger.Printf("  -> Dependency: %s", depName)
			
			depOpts := *opts // Shallow copy
			depOpts.Package = depName
			depOpts.Version = "" // Always use latest for deps to avoid version conflict hell
			
			if err := pm.installRecursive(ctx, &depOpts, visited); err != nil {
				// Warn but proceed. This handles cases where a dependency is 
				// a virtual package (e.g. "so:libssl.so.3") that we can't resolve yet.
				pm.logger.Printf("  ⚠️  Warning: failed to install dependency %s: %v", depName, err)
			}
		}
	}

	// 3. Download package
	pm.logger.Printf("Downloading %s...", pkgInfo.Package)
	apkPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s-%s.apk", pkgInfo.Package, pkgInfo.Version))

	// Construct download URL: {repo}/{branch}/{repository}/{arch}/{package}-{version}.apk
	// Note: We need to know which repository (main/community) the package came from.
	// The findPackage method doesn't return the repo, but we can try both or cache it.
	// For simplicity in this structure, we assume main, then fallback if download fails?
	// Actually, let's try to determine it or try both URLs.
	
	// Try constructing URL. Since we don't store which repo the package came from in PackageInfo,
	// we have to try both standard locations.
	
	repos := []string{pm.config.Repository, "main", "community"}
	var downloadErr error
	downloaded := false

	for _, repo := range repos {
		url := fmt.Sprintf("%s/%s/%s/%s/%s-%s.apk",
			pm.config.RepositoryURL,
			pm.config.Branch,
			repo,
			opts.Architecture,
			pkgInfo.Package,
			pkgInfo.Version)
		
		if err := pm.downloadPackage(ctx, url, apkPath); err == nil {
			downloaded = true
			break
		} else {
			downloadErr = err
		}
	}

	if !downloaded {
		return fmt.Errorf("downloading package %s failed: %w", pkgInfo.Package, downloadErr)
	}

	// 4. Verify hash
	if opts.VerifyHash && pkgInfo.Checksum != "" {
		if err := pm.verifyFileHash(apkPath, pkgInfo.Checksum); err != nil {
			return fmt.Errorf("hash verification failed for %s: %w", pkgInfo.Package, err)
		}
	}

	// 5. Extract package
	if opts.Extract {
		pm.logger.Printf("Extracting %s...", pkgInfo.Package)
		if err := pm.extractAPKPackage(apkPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting package %s: %w", pkgInfo.Package, err)
		}

		// 6. Cleanup archive if requested
		if !opts.KeepArchive {
			os.Remove(apkPath)
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

	// Clear cache before updating
	pm.cache.packages = make(map[string]*PackageInfo)

	// Common Alpine repositories to search
	repositories := []string{"main", "community"}

	totalPackages := 0
	var lastErr error

	for _, repo := range repositories {
		// Construct URL for APKINDEX.tar.gz
		url := fmt.Sprintf("%s/%s/%s/%s/APKINDEX.tar.gz",
			pm.config.RepositoryURL,
			pm.config.Branch,
			repo,
			arch)

		pm.logger.Printf("  Fetching %s repository: %s", repo, url)

		// Download APKINDEX
		resp, err := pm.client.Get(ctx, url)
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to fetch %s repository: %v", repo, err)
			lastErr = err
			continue
		}

		// Parse packages
		packages, err := ParseAPKINDEX(resp.Body)
		resp.Body.Close()
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to parse %s repository: %v", repo, err)
			lastErr = err
			continue
		}

		pm.logger.Printf("  ✓ Parsed %d packages from %s", len(packages), repo)

		// Add to cache
		for _, pkg := range packages {
			key := fmt.Sprintf("%s_%s_%s", pkg.Package, pkg.Architecture, repo)
			pm.cache.packages[key] = pkg
		}

		totalPackages += len(packages)
	}

	if totalPackages == 0 {
		if lastErr != nil {
			return fmt.Errorf("failed to fetch any packages: %w", lastErr)
		}
		return fmt.Errorf("no packages found in any repository")
	}

	pm.logger.Printf("  Total packages indexed: %d", totalPackages)
	pm.cache.lastUpdate = time.Now()

	return nil
}

// findPackage finds a package in the cache across all repositories
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	// Search order: main, community
	repositories := []string{"main", "community"}

	for _, repo := range repositories {
		// Try exact architecture first
		key := fmt.Sprintf("%s_%s_%s", name, arch, repo)
		if pkg, ok := pm.cache.packages[key]; ok {
			if version == "" || pkg.Version == version {
				return pkg, nil
			}
		}

		// Try "noarch" architecture
		key = fmt.Sprintf("%s_%s_%s", name, ArchNoarch, repo)
		if pkg, ok := pm.cache.packages[key]; ok {
			if version == "" || pkg.Version == version {
				return pkg, nil
			}
		}
	}

	if version != "" {
		return nil, fmt.Errorf("package %s version %s not found", name, version)
	}
	return nil, fmt.Errorf("package %s not found", name)
}

// downloadPackage downloads an .apk package
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

// verifyFileHash verifies the SHA1 hash of a file (Alpine uses SHA1 in C: field)
func (pm *PackageManager) verifyFileHash(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("computing hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	// Alpine uses Q prefix for SHA1 hashes in the format Q1{hash}
	expectedHashClean := strings.TrimPrefix(expectedHash, "Q1")

	if !strings.EqualFold(actualHash, expectedHashClean) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHashClean, actualHash)
	}

	return nil
}

// extractAPKPackage extracts an .apk package (which is a tar.gz archive)
func (pm *PackageManager) extractAPKPackage(apkPath, installPath string) error {
	// Open the .apk file
	f, err := os.Open(apkPath)
	if err != nil {
		return fmt.Errorf("opening .apk file: %w", err)
	}
	defer f.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract each entry
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Skip special Alpine metadata files
		if strings.HasPrefix(header.Name, ".PKGINFO") ||
			strings.HasPrefix(header.Name, ".SIGN.") ||
			strings.HasPrefix(header.Name, ".post-") ||
			strings.HasPrefix(header.Name, ".pre-") {
			continue
		}

		targetPath := filepath.Join(installPath, header.Name)

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