// pkg/apt/manager.go
package apt

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
	"github.com/ulikunitz/xz"
)

// NewPackageManager creates a new Ubuntu package manager
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
	if cfg.PortsURL == "" {
		cfg.PortsURL = DefaultPortsURL
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
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "apt")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	// Setup logger
	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[APT] ", log.LstdFlags)
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
		pm.logger.Printf("Initialized Ubuntu APT PackageManager")
		pm.logger.Printf("  Repository: %s", cfg.RepositoryURL)
		pm.logger.Printf("  Release: %s", cfg.Release)
		pm.logger.Printf("  Component: %s", cfg.Component)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  CachePath: %s", cfg.CachePath)
	}

	return pm
}

// Download downloads and installs an Ubuntu package
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
	pm.logger.Printf("    Description: %s", strings.Split(pkgInfo.Description, "\n")[0])
	pm.logger.Printf("    Size: %d bytes", pkgInfo.Size)

	// 3. Download package
	pm.logger.Printf("Step 3: Downloading package...")
	debPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s_%s_%s.deb", pkgInfo.Package, pkgInfo.Version, pkgInfo.Architecture))

	// Determine correct repository URL
	repoURL := pm.config.RepositoryURL
	if opts.Architecture.UsesPortsRepo() {
		repoURL = pm.config.PortsURL
		pm.logger.Printf("  Using ports repository for %s architecture", opts.Architecture)
	}

	downloadURL := fmt.Sprintf("%s/%s", repoURL, pkgInfo.Filename)
	if err := pm.downloadPackage(ctx, downloadURL, debPath); err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}
	pm.logger.Printf("  ✓ Download complete")

	// 4. Verify hash
	if opts.VerifyHash && pkgInfo.SHA256 != "" {
		pm.logger.Printf("Step 4: Verifying SHA256 hash...")
		if err := pm.verifyFileHash(debPath, pkgInfo.SHA256); err != nil {
			return fmt.Errorf("hash verification failed: %w", err)
		}
		pm.logger.Printf("  ✓ Hash verified")
	} else {
		pm.logger.Printf("Step 4: Skipping hash verification")
	}

	// 5. Extract package
	if opts.Extract {
		pm.logger.Printf("Step 5: Extracting package...")
		if err := pm.extractDebPackage(debPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting package: %w", err)
		}
		pm.logger.Printf("  ✓ Extraction complete")

		// 6. Cleanup archive if requested
		if !opts.KeepArchive {
			pm.logger.Printf("Step 6: Removing archive file...")
			if err := os.Remove(debPath); err != nil {
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

	// Determine correct repository URL based on architecture
	repoURL := pm.config.RepositoryURL
	if arch.UsesPortsRepo() {
		repoURL = pm.config.PortsURL
		pm.logger.Printf("  Using ports repository for %s architecture", arch)
	}

	// Clear cache before updating
	pm.cache.packages = make(map[string]*PackageInfo)

	// Common Ubuntu components to search
	components := []string{"main", "universe", "restricted", "multiverse"}
	
	totalPackages := 0
	for _, component := range components {
		// Construct URL for Packages.gz
		url := fmt.Sprintf("%s/dists/%s/%s/binary-%s/Packages.gz",
			repoURL,
			pm.config.Release,
			component,
			arch)

		pm.logger.Printf("  Fetching %s component: %s", component, url)

		// Download and decompress
		reader, err := pm.client.GetGzipped(ctx, url)
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to fetch %s component: %v", component, err)
			continue // Skip this component if it fails
		}

		// Parse packages
		packages, err := ParsePackages(reader)
		reader.Close()
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to parse %s component: %v", component, err)
			continue
		}

		pm.logger.Printf("  ✓ Parsed %d packages from %s", len(packages), component)

		// Add to cache
		for _, pkg := range packages {
			key := fmt.Sprintf("%s_%s_%s", pkg.Package, pkg.Architecture, component)
			pm.cache.packages[key] = pkg
		}
		
		totalPackages += len(packages)
	}

	pm.logger.Printf("  Total packages indexed: %d", totalPackages)
	pm.cache.lastUpdate = time.Now()

	return nil
}

// findPackage finds a package in the cache across all components
func (pm *PackageManager) findPackage(name, version string, arch Architecture) (*PackageInfo, error) {
	// Search order: main, universe, restricted, multiverse
	components := []string{"main", "universe", "restricted", "multiverse"}
	
	for _, component := range components {
		// Try exact architecture first
		key := fmt.Sprintf("%s_%s_%s", name, arch, component)
		if pkg, ok := pm.cache.packages[key]; ok {
			if version == "" || pkg.Version == version {
				pm.logger.Printf("  Found in component: %s", component)
				return pkg, nil
			}
		}

		// Try "all" architecture
		key = fmt.Sprintf("%s_%s_%s", name, ArchAll, component)
		if pkg, ok := pm.cache.packages[key]; ok {
			if version == "" || pkg.Version == version {
				pm.logger.Printf("  Found in component: %s", component)
				return pkg, nil
			}
		}
	}

	if version != "" {
		return nil, fmt.Errorf("package %s version %s not found for architecture %s", name, version, arch)
	}
	return nil, fmt.Errorf("package %s not found for architecture %s (searched components: main, universe, restricted, multiverse)", name, arch)
}

// downloadPackage downloads a .deb package
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

// verifyFileHash verifies the SHA256 hash of a file
func (pm *PackageManager) verifyFileHash(filePath, expectedHash string) error {
	pm.logger.Printf("Computing SHA256 hash of: %s", filePath)

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

	pm.logger.Printf("  Expected: %s", expectedHash)
	pm.logger.Printf("  Actual:   %s", actualHash)

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	pm.logger.Printf("  ✓ Hashes match!")
	return nil
}

// extractDebPackage extracts a .deb package using the ar and tar formats
func (pm *PackageManager) extractDebPackage(debPath, installPath string) error {
	pm.logger.Printf("Extracting .deb package: %s -> %s", debPath, installPath)

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

		pm.logger.Printf("  Found ar member: %s (%d bytes)", header.Name, header.Size)

		// Look for data.tar.* (data.tar.xz, data.tar.gz, data.tar.zst, etc.)
		if strings.HasPrefix(header.Name, "data.tar") {
			pm.logger.Printf("  Extracting data archive: %s", header.Name)
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
		pm.logger.Printf("  Using gzip decompression")
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		tarReader = tar.NewReader(gzReader)
	} else if strings.HasSuffix(name, ".xz") {
		pm.logger.Printf("  Using xz decompression")
		xzReader, err := xz.NewReader(r)
		if err != nil {
			return fmt.Errorf("creating xz reader: %w", err)
		}
		tarReader = tar.NewReader(xzReader)
	} else if strings.HasSuffix(name, ".zst") {
		// zstd compression
		return fmt.Errorf("zstd compression not yet supported")
	} else {
		// Assume uncompressed tar
		pm.logger.Printf("  Using uncompressed tar")
		tarReader = tar.NewReader(r)
	}

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
			dirCount++
			pm.logger.Printf("    📁 %s/", cleanPath)

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
			pm.logger.Printf("    🔗 %s -> %s", cleanPath, header.Linkname)

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
			pm.logger.Printf("    📄 %s (%d bytes)%s", cleanPath, header.Size, execFlag)

		default:
			pm.logger.Printf("    ⚠️  Skipping unsupported file type %v for %s", header.Typeflag, cleanPath)
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