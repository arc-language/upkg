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

// Download downloads and installs an Alpine package
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
	pm.logger.Printf("  ✓ Package found: %s %s", pkgInfo.Package, pkgInfo.Version)
	pm.logger.Printf("    Description: %s", pkgInfo.Description)
	pm.logger.Printf("    Size: %d bytes", pkgInfo.PackageSize)

	// 3. Download package
	pm.logger.Printf("Step 3: Downloading package...")
	apkPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s-%s.apk", pkgInfo.Package, pkgInfo.Version))

	// Construct download URL: {repo}/{branch}/{repository}/{arch}/{package}-{version}.apk
	downloadURL := fmt.Sprintf("%s/%s/%s/%s/%s-%s.apk",
		pm.config.RepositoryURL,
		pm.config.Branch,
		pm.config.Repository,
		opts.Architecture,
		pkgInfo.Package,
		pkgInfo.Version)

	if err := pm.downloadPackage(ctx, downloadURL, apkPath); err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}
	pm.logger.Printf("  ✓ Download complete")

	// 4. Verify hash
	if opts.VerifyHash && pkgInfo.Checksum != "" {
		pm.logger.Printf("Step 4: Verifying SHA1 hash...")
		if err := pm.verifyFileHash(apkPath, pkgInfo.Checksum); err != nil {
			return fmt.Errorf("hash verification failed: %w", err)
		}
		pm.logger.Printf("  ✓ Hash verified")
	} else {
		pm.logger.Printf("Step 4: Skipping hash verification")
	}

	// 5. Extract package
	if opts.Extract {
		pm.logger.Printf("Step 5: Extracting package...")
		if err := pm.extractAPKPackage(apkPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting package: %w", err)
		}
		pm.logger.Printf("  ✓ Extraction complete")

		// 6. Cleanup archive if requested
		if !opts.KeepArchive {
			pm.logger.Printf("Step 6: Removing archive file...")
			if err := os.Remove(apkPath); err != nil {
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

	// Common Alpine repositories to search
	repositories := []string{"main", "community"}

	totalPackages := 0
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
			continue
		}

		// Parse packages
		packages, err := ParseAPKINDEX(resp.Body)
		resp.Body.Close()
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to parse %s repository: %v", repo, err)
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
				pm.logger.Printf("  Found in repository: %s", repo)
				return pkg, nil
			}
		}

		// Try "noarch" architecture
		key = fmt.Sprintf("%s_%s_%s", name, ArchNoarch, repo)
		if pkg, ok := pm.cache.packages[key]; ok {
			if version == "" || pkg.Version == version {
				pm.logger.Printf("  Found in repository: %s", repo)
				return pkg, nil
			}
		}
	}

	if version != "" {
		return nil, fmt.Errorf("package %s version %s not found for architecture %s", name, version, arch)
	}
	return nil, fmt.Errorf("package %s not found for architecture %s (searched repositories: main, community)", name, arch)
}

// downloadPackage downloads an .apk package
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

// verifyFileHash verifies the SHA1 hash of a file (Alpine uses SHA1 in C: field)
func (pm *PackageManager) verifyFileHash(filePath, expectedHash string) error {
	pm.logger.Printf("Computing SHA1 hash of: %s", filePath)

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

	pm.logger.Printf("  Expected: %s", expectedHash)
	pm.logger.Printf("  Actual:   %s", actualHash)

	// Alpine uses Q prefix for SHA1 hashes in the format Q1{hash}
	expectedHashClean := strings.TrimPrefix(expectedHash, "Q1")

	if !strings.EqualFold(actualHash, expectedHashClean) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHashClean, actualHash)
	}

	pm.logger.Printf("  ✓ Hashes match!")
	return nil
}

// extractAPKPackage extracts an .apk package (which is a tar.gz archive)
func (pm *PackageManager) extractAPKPackage(apkPath, installPath string) error {
	pm.logger.Printf("Extracting .apk package: %s -> %s", apkPath, installPath)

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

	// Track statistics
	fileCount := 0
	dirCount := 0
	symlinkCount := 0

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
			pm.logger.Printf("  Skipping metadata: %s", header.Name)
			continue
		}

		targetPath := filepath.Join(installPath, header.Name)

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, err)
			}
			dirCount++
			pm.logger.Printf("  📁 %s/", header.Name)

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory for symlink: %w", err)
			}
			// Remove existing symlink if it exists
			os.Remove(targetPath)
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", targetPath, header.Linkname, err)
			}
			symlinkCount++
			pm.logger.Printf("  🔗 %s -> %s", header.Name, header.Linkname)

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

			fileCount++
			execFlag := ""
			if header.Mode&0111 != 0 {
				execFlag = " (executable)"
			}
			pm.logger.Printf("  📄 %s (%d bytes)%s", header.Name, header.Size, execFlag)

		default:
			pm.logger.Printf("  ⚠️  Skipping unsupported file type %v for %s", header.Typeflag, header.Name)
		}
	}

	pm.logger.Printf("  ✓ Extraction complete:")
	pm.logger.Printf("    - %d files", fileCount)
	pm.logger.Printf("    - %d directories", dirCount)
	pm.logger.Printf("    - %d symlinks", symlinkCount)

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