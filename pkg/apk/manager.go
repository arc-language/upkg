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
			providers:     make(map[string][]*PackageInfo),
			cacheDuration: 30 * time.Minute,
		},
	}

	if cfg.Debug {
		pm.logger.Printf("Initialized Alpine APK PackageManager")
		pm.logger.Printf("  Branch: %s", cfg.Branch)
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
	}

	// 1. Update package index once
	if err := pm.updatePackageIndex(ctx, opts.Architecture); err != nil {
		return fmt.Errorf("updating package index: %w", err)
	}

	// Track installed packages to avoid loops
	visited := make(map[string]bool)

	// Start recursive installation
	return pm.installRecursive(ctx, opts, visited)
}

// installRecursive handles the actual download, dependency resolution, and extraction
func (pm *PackageManager) installRecursive(ctx context.Context, opts *DownloadOptions, visited map[string]bool) error {
	// 1. Find package info (resolves virtual names like "so:libssl.so.3" to "libssl3")
	pkgInfo, err := pm.findPackage(opts.Package, opts.Version, opts.Architecture)
	if err != nil {
		// Alpine dependencies often use virtual names. If we can't find it even after
		// our new resolution logic, it's a real error.
		return fmt.Errorf("resolving package %s: %w", opts.Package, err)
	}

	// Skip if already visited (use resolved real name)
	if visited[pkgInfo.Package] {
		return nil
	}
	visited[pkgInfo.Package] = true

	pm.logger.Printf("Processing package: %s (resolved from %s)", pkgInfo.Package, opts.Package)

	// 2. Resolve and install dependencies FIRST
	if len(pkgInfo.Depends) > 0 {
		for _, depName := range pkgInfo.Depends {
			// Skip self-references
			if depName == pkgInfo.Package {
				continue
			}
			
			pm.logger.Printf("  -> Dependency: %s", depName)
			
			depOpts := *opts // Shallow copy
			depOpts.Package = depName
			depOpts.Version = "" // Use latest/resolved version for deps
			
			if err := pm.installRecursive(ctx, &depOpts, visited); err != nil {
				pm.logger.Printf("  ⚠️  Warning: failed to install dependency %s: %v", depName, err)
			}
		}
	}

	// 3. Download package
	pm.logger.Printf("Downloading %s...", pkgInfo.Package)
	apkPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s-%s.apk", pkgInfo.Package, pkgInfo.Version))

	// Construct URL using the package's specific repository
	// URL: {base}/{branch}/{repo}/{arch}/{pkg}-{ver}.apk
	url := fmt.Sprintf("%s/%s/%s/%s/%s-%s.apk",
		pm.config.RepositoryURL,
		pm.config.Branch,
		pkgInfo.Repository,
		opts.Architecture,
		pkgInfo.Package,
		pkgInfo.Version)
	
	if err := pm.downloadPackage(ctx, url, apkPath); err != nil {
		return fmt.Errorf("downloading package %s: %w", pkgInfo.Package, err)
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

// findPackage finds a package using the cache and provider map
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	// 1. Try exact match in package map
	if pkg, ok := pm.cache.packages[name]; ok {
		if version == "" || pkg.Version == version {
			return pkg, nil
		}
	}

	// 2. Try resolving as a virtual package (Provides)
	if pkg, err := pm.resolveVirtualPackage(name); err == nil {
		if version == "" || pkg.Version == version {
			return pkg, nil
		}
	}

	return nil, fmt.Errorf("package %s not found", name)
}

// downloadPackage downloads an .apk package
func (pm *PackageManager) downloadPackage(ctx context.Context, url, destPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Check if already downloaded
	if _, err := os.Stat(destPath); err == nil {
		return nil
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
		os.Remove(destPath) // Clean up partial
		return fmt.Errorf("downloading: %w", err)
	}

	return nil
}

// verifyFileHash verifies the SHA1 hash of a file
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

// extractAPKPackage extracts an .apk package
func (pm *PackageManager) extractAPKPackage(apkPath, installPath string) error {
	f, err := os.Open(apkPath)
	if err != nil {
		return fmt.Errorf("opening .apk file: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Skip metadata
		if strings.HasPrefix(header.Name, ".PKGINFO") ||
			strings.HasPrefix(header.Name, ".SIGN.") ||
			strings.HasPrefix(header.Name, ".post-") ||
			strings.HasPrefix(header.Name, ".pre-") {
			continue
		}

		targetPath := filepath.Join(installPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating dir: %w", err)
			}
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			os.Remove(targetPath)
			os.Symlink(header.Linkname, targetPath)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			io.Copy(outFile, tarReader)
			outFile.Close()
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