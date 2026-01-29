// pkg/choco/manager.go
package choco

import (
	"archive/zip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewPackageManager creates a new Chocolatey package manager
func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.RepositoryURL == "" {
		cfg.RepositoryURL = DefaultRepositoryURL
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.CachePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "choco")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	// Setup logger
	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[CHOCO] ", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}
	}

	pm := &PackageManager{
		client: NewClientWithTimeout(cfg.Timeout),
		config: cfg,
		logger: logger,
	}

	if cfg.Debug {
		pm.logger.Printf("Initialized Chocolatey PackageManager")
		pm.logger.Printf("  Repository: %s", cfg.RepositoryURL)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  CachePath: %s", cfg.CachePath)
	}

	return pm
}

// Download downloads and extracts a Chocolatey package
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts == nil || opts.Package == "" {
		return fmt.Errorf("Package is required in DownloadOptions")
	}

	pm.logger.Printf("Starting download for package: %s", opts.Package)

	pm.logger.Printf("Download options:")
	pm.logger.Printf("  Package: %s", opts.Package)
	pm.logger.Printf("  Version: %s", opts.Version)
	pm.logger.Printf("  Extract: %v", opts.Extract)
	pm.logger.Printf("  KeepArchive: %v", opts.KeepArchive)
	pm.logger.Printf("  VerifyHash: %v", opts.VerifyHash)

	// 1. Get package info
	pm.logger.Printf("Step 1: Getting package info...")
	pkgInfo, err := pm.getPackageInfo(ctx, opts.Package, opts.Version)
	if err != nil {
		return fmt.Errorf("getting package info: %w", err)
	}
	pm.logger.Printf("  ✓ Package found: %s %s", pkgInfo.ID, pkgInfo.Version)
	pm.logger.Printf("    Title: %s", pkgInfo.Title)
	pm.logger.Printf("    Size: %d bytes", pkgInfo.PackageSize)

	// 2. Download package
	pm.logger.Printf("Step 2: Downloading package...")
	downloadURL := fmt.Sprintf("%s/package/%s/%s", pm.config.RepositoryURL, pkgInfo.ID, pkgInfo.Version)
	
	nupkgPath := filepath.Join(pm.config.CachePath, "downloads",
		fmt.Sprintf("%s.%s.nupkg", pkgInfo.ID, pkgInfo.Version))

	if err := pm.downloadPackage(ctx, downloadURL, nupkgPath); err != nil {
		return fmt.Errorf("downloading package: %w", err)
	}
	pm.logger.Printf("  ✓ Download complete")

	// 3. Verify hash
	if opts.VerifyHash && pkgInfo.PackageHash != "" {
		pm.logger.Printf("Step 3: Verifying checksum...")
		if err := pm.verifyFileHash(nupkgPath, pkgInfo.PackageHash, pkgInfo.PackageHashAlgo); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		pm.logger.Printf("  ✓ Checksum verified")
	} else {
		pm.logger.Printf("Step 3: Skipping checksum verification")
	}

	// 4. Extract package
	if opts.Extract {
		pm.logger.Printf("Step 4: Extracting package...")
		extractPath := filepath.Join(pm.config.InstallPath, pkgInfo.ID)
		if err := pm.extractNupkg(nupkgPath, extractPath); err != nil {
			return fmt.Errorf("extracting package: %w", err)
		}
		pm.logger.Printf("  ✓ Extraction complete")

		// 5. Cleanup archive if requested
		if !opts.KeepArchive {
			pm.logger.Printf("Step 5: Removing archive file...")
			if err := os.Remove(nupkgPath); err != nil {
				pm.logger.Printf("  ⚠️  Warning: failed to remove archive: %v", err)
			} else {
				pm.logger.Printf("  ✓ Archive removed")
			}
		} else {
			pm.logger.Printf("Step 5: Keeping archive file as requested")
		}
	} else {
		pm.logger.Printf("Step 4: Skipping extraction (Extract=false)")
	}

	pm.logger.Printf("✓ Package %s installed successfully", opts.Package)
	return nil
}

// getPackageInfo retrieves package information from the repository
func (pm *PackageManager) getPackageInfo(ctx context.Context, packageID, version string) (*PackageInfo, error) {
	var url string
	if version != "" {
		// Get specific version
		url = fmt.Sprintf("%s/Packages(Id='%s',Version='%s')", pm.config.RepositoryURL, packageID, version)
	} else {
		// Get latest version - FIXED: use tolower(Id) and proper filter syntax
		url = fmt.Sprintf("%s/Packages()?$filter=(tolower(Id) eq '%s') and IsLatestVersion&$top=1", pm.config.RepositoryURL, packageID)
	}

	pm.logger.Printf("  Fetching package metadata: %s", url)

	resp, err := pm.client.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetching package info: %w", err)
	}
	defer resp.Body.Close()

	packages, err := ParseFeed(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing package info: %w", err)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("package %s not found", packageID)
	}

	return packages[0], nil
}

// downloadPackage downloads a .nupkg file
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
func (pm *PackageManager) verifyFileHash(filePath, expectedHash, hashAlgo string) error {
	pm.logger.Printf("Computing %s checksum of: %s", hashAlgo, filePath)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// Chocolatey uses SHA512 base64 encoded
	if !strings.EqualFold(hashAlgo, "SHA512") {
		pm.logger.Printf("  Skipping verification: unsupported hash algorithm %s", hashAlgo)
		return nil
	}

	hasher := sha512.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("computing hash: %w", err)
	}

	actualHash := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	pm.logger.Printf("  Expected: %s", expectedHash)
	pm.logger.Printf("  Actual:   %s", actualHash)

	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	pm.logger.Printf("  ✓ Hashes match!")
	return nil
}

// extractNupkg extracts a .nupkg file (which is a ZIP archive)
func (pm *PackageManager) extractNupkg(nupkgPath, extractPath string) error {
	pm.logger.Printf("Extracting .nupkg package: %s -> %s", nupkgPath, extractPath)

	// Ensure extract directory exists
	if err := os.MkdirAll(extractPath, 0755); err != nil {
		return fmt.Errorf("creating extract directory: %w", err)
	}

	// Open the ZIP file
	reader, err := zip.OpenReader(nupkgPath)
	if err != nil {
		return fmt.Errorf("opening nupkg: %w", err)
	}
	defer reader.Close()

	// Extract all files
	for _, file := range reader.File {
		pm.logger.Printf("  Extracting: %s", file.Name)

		// Construct full path
		path := filepath.Join(extractPath, file.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(path, filepath.Clean(extractPath)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// Create directory
			os.MkdirAll(path, file.Mode())
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		// Extract file
		if err := pm.extractFile(file, path); err != nil {
			return fmt.Errorf("extracting file %s: %w", file.Name, err)
		}
	}

	pm.logger.Printf("  ✓ Extraction complete")
	return nil
}

// extractFile extracts a single file from the ZIP
func (pm *PackageManager) extractFile(file *zip.File, destPath string) error {
	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

// GetPackageInfo retrieves information about a package
func (pm *PackageManager) GetPackageInfo(ctx context.Context, name string) (*PackageInfo, error) {
	return pm.getPackageInfo(ctx, name, "")
}

// SearchPackages searches for packages
func (pm *PackageManager) SearchPackages(ctx context.Context, query string) ([]*PackageInfo, error) {
	// FIXED: proper Search() endpoint parameters
	url := fmt.Sprintf("%s/Search()?$filter=IsLatestVersion&$orderby=Id&searchTerm='%s'&targetFramework=''&includePrerelease=false&$skip=0&$top=30&semVerLevel=2.0.0", 
		pm.config.RepositoryURL, query)

	pm.logger.Printf("Searching packages: %s", url)

	resp, err := pm.client.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}
	defer resp.Body.Close()

	packages, err := ParseFeed(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}

	return packages, nil
}