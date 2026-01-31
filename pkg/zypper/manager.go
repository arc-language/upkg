// pkg/zypper/manager.go
package zypper

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cavaliergopher/cpio"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	if cfg.MirrorURL == "" {
		cfg.MirrorURL = DefaultMirror
	}
	if cfg.Distribution == "" {
		cfg.Distribution = DefaultDistribution
	}
	if len(cfg.Repos) == 0 {
		cfg.Repos = DefaultRepos
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.CachePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "zypper")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[ZYPPER] ", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}
	}

	return &PackageManager{
		client: NewClient(cfg.Timeout),
		config: cfg,
		logger: logger,
		cache: &PackageCache{
			packages:      make(map[string]*PackageInfo),
			cacheDuration: 30 * time.Minute,
		},
	}
}

// Download downloads and installs a package and its dependencies
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts.Architecture == "" {
		opts.Architecture = DefaultArch
	}

	pm.logger.Printf("Starting operation for package: %s", opts.Package)

	// 1. Update/Sync DB once at the top level
	if err := pm.updateDB(ctx, opts.Architecture); err != nil {
		return err
	}

	// Track installed packages to avoid loops
	visited := make(map[string]bool)

	return pm.installRecursive(ctx, opts, visited)
}

// installRecursive handles the actual download, dependency resolution, and extraction
func (pm *PackageManager) installRecursive(ctx context.Context, opts *DownloadOptions, visited map[string]bool) error {
	// Skip if already visited/installed in this transaction
	if visited[opts.Package] {
		return nil
	}
	visited[opts.Package] = true

	// 1. Find Package
	pkg, err := pm.findPackage(opts.Package, opts.Version)
	if err != nil {
		// If we can't find a dependency, we log a warning but don't fail hard,
		// as it might be a virtual package or capability provided by the system.
		pm.logger.Printf("  ⚠️ Warning: Could not find package/dependency '%s': %v", opts.Package, err)
		return nil
	}
	
	pm.logger.Printf("Processing: %s %s", pkg.Name, pkg.Version)

	// 2. Resolve Dependencies Recursive Loop
	if len(pkg.Dependencies) > 0 {
		pm.logger.Printf("  Resolving dependencies for %s...", pkg.Name)
		for _, dep := range pkg.Dependencies {
			pm.logger.Printf("    -> Dependency: %s", dep.Name)
			
			// Create options for the dependency
			depOpts := *opts // Shallow copy
			depOpts.Package = dep.Name
			depOpts.Version = "" // Always use latest available for dependencies for now
			
			// Recurse
			if err := pm.installRecursive(ctx, &depOpts, visited); err != nil {
				pm.logger.Printf("    ⚠️ Warning: Failed to install dependency %s: %v", dep.Name, err)
			}
		}
	}

	// 3. Download
	// URL Construction: Mirror / Distribution / RepoPath / LocationFromXML
	repoBaseURL := fmt.Sprintf("%s/%s/%s", pm.config.MirrorURL, pm.config.Distribution, pkg.Repository)
	
	// Ensure no double slashes if Repo is handled differently in caching
	downloadURL := fmt.Sprintf("%s/%s", repoBaseURL, pkg.Location)

	destPath := filepath.Join(pm.config.CachePath, "downloads", filepath.Base(pkg.Location))

	pm.logger.Printf("  Downloading %s...", pkg.Name)
	if err := pm.downloadFile(ctx, downloadURL, destPath); err != nil {
		return fmt.Errorf("downloading %s: %w", pkg.Name, err)
	}

	// 4. Verify
	if opts.VerifyHash && pkg.Checksum != "" {
		if err := pm.verifyHash(destPath, pkg.Checksum, pkg.ChecksumType); err != nil {
			return err
		}
	}

	// 5. Extract
	if opts.Extract {
		pm.logger.Printf("  Extracting %s...", pkg.Name)
		if err := pm.extractRPM(destPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting %s: %w", pkg.Name, err)
		}
		if !opts.KeepArchive {
			os.Remove(destPath)
		}
	}

	pm.logger.Printf("✓ Installed %s", pkg.Name)
	return nil
}

func (pm *PackageManager) updateDB(ctx context.Context, arch string) error {
	if len(pm.cache.packages) > 0 && time.Since(pm.cache.lastUpdate) < pm.cache.cacheDuration {
		return nil
	}

	pm.logger.Printf("Syncing databases...")
	pm.cache.packages = make(map[string]*PackageInfo)

	for _, repoPath := range pm.config.Repos {
		// 1. Get repomd.xml
		baseURL := fmt.Sprintf("%s/%s/%s", pm.config.MirrorURL, pm.config.Distribution, repoPath)
		repomdURL := fmt.Sprintf("%s/repodata/repomd.xml", baseURL)

		pm.logger.Printf("  Fetching repomd: %s", repomdURL)
		
		repomdBody, err := pm.client.Get(ctx, repomdURL)
		if err != nil {
			pm.logger.Printf("    ⚠️ Failed to fetch repomd for %s: %v", repoPath, err)
			continue
		}
		
		primaryLoc, err := ParseRepomd(repomdBody)
		repomdBody.Close()
		if err != nil {
			pm.logger.Printf("    ⚠️ Failed to parse repomd for %s: %v", repoPath, err)
			continue
		}

		// 2. Get Primary XML
		primaryURL := fmt.Sprintf("%s/%s", baseURL, primaryLoc)
		pm.logger.Printf("    Fetching primary: %s", primaryURL)

		primaryBody, err := pm.client.Get(ctx, primaryURL)
		if err != nil {
			pm.logger.Printf("    ⚠️ Failed to fetch primary XML: %v", err)
			continue
		}

		// Pass primaryLoc (filename) so parser knows to use zstd or gzip
		pkgs, err := ParsePrimary(primaryBody, primaryLoc, repoPath)
		primaryBody.Close()
		if err != nil {
			pm.logger.Printf("    ⚠️ Failed to parse primary XML: %v", err)
			continue
		}

		// 3. Filter by architecture and add to cache
		count := 0
		for _, p := range pkgs {
			// Basic arch filtering
			if p.Architecture == "noarch" || p.Architecture == arch {
				// Simple Last-Write-Wins for versioning in this map
				pm.cache.packages[p.Name] = p
				count++
			}
		}
		pm.logger.Printf("    Indexed %d packages from %s", count, repoPath)
	}

	pm.cache.lastUpdate = time.Now()
	return nil
}

func (pm *PackageManager) findPackage(name, version string) (*PackageInfo, error) {
	if pkg, ok := pm.cache.packages[name]; ok {
		if version == "" || pkg.Version == version {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("package %s not found", name)
}

func (pm *PackageManager) downloadFile(ctx context.Context, url, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = pm.client.Download(ctx, url, f)
	return err
}

func (pm *PackageManager) verifyHash(path, expected, hashType string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var sum []byte

	if hashType == "sha512" {
		h := sha512.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		sum = h.Sum(nil)
	} else if hashType == "sha256" {
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		sum = h.Sum(nil)
	} else {
		// PM shouldn't fail if the XML uses a hash we don't support yet, just warn
		pm.logger.Printf("  ⚠️ Skipping verification (unsupported hash type: %s)", hashType)
		return nil
	}

	actual := hex.EncodeToString(sum)
	if actual != expected {
		return fmt.Errorf("hash mismatch: want %s, got %s", expected, actual)
	}
	
	return nil
}

// extractRPM extracts an RPM package.
func (pm *PackageManager) extractRPM(rpmPath, dest string) error {
	f, err := os.Open(rpmPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// 1. Find the start of the compressed CPIO archive.
	// We look for GZIP (1f 8b), Zstd (28 b5 2f fd), or XZ (fd 37 7a 58 5a 00)
	// Scanning the first 1MB should be enough to skip RPM headers.
	scanSize := 1024 * 1024 // 1MB buffer
	buf := make([]byte, scanSize) 
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}

	var offset int64 = -1
	var format string

	for i := 0; i < n-6; i++ {
		// Check for GZIP
		if buf[i] == 0x1f && buf[i+1] == 0x8b {
			offset = int64(i)
			format = "gzip"
			break
		}
		// Check for ZSTD
		if buf[i] == 0x28 && buf[i+1] == 0xb5 && buf[i+2] == 0x2f && buf[i+3] == 0xfd {
			offset = int64(i)
			format = "zstd"
			break
		}
		// Check for XZ
		if buf[i] == 0xfd && buf[i+1] == 0x37 && buf[i+2] == 0x7a && buf[i+3] == 0x58 && buf[i+4] == 0x5a && buf[i+5] == 0x00 {
			offset = int64(i)
			format = "xz"
			break
		}
	}

	if offset == -1 {
		return fmt.Errorf("could not find compressed archive within RPM (scanned %d bytes)", n)
	}

	f.Seek(offset, 0)

	// 2. Decompress
	var reader io.Reader
	if format == "gzip" {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		reader = gz
	} else if format == "zstd" {
		zs, err := zstd.NewReader(f)
		if err != nil {
			return err
		}
		defer zs.Close()
		reader = zs
	} else if format == "xz" {
		x, err := xz.NewReader(f)
		if err != nil {
			return err
		}
		reader = x
	}

	// 3. Extract CPIO
	cpioReader := cpio.NewReader(reader)
	
	for {
		header, err := cpioReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading cpio: %w", err)
		}

		// Skip metadata/directories if needed, or create them
		target := filepath.Join(dest, header.Name)
		
		if header.Mode.IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		// Ensure parent exists
		os.MkdirAll(filepath.Dir(target), 0755)

		if header.Mode.IsRegular() {
			perm := os.FileMode(header.Mode & 0777)
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, cpioReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		} else if (header.Mode & 0170000) == 0120000 { // Symlink
			if header.Linkname != "" {
				os.Remove(target)
				os.Symlink(header.Linkname, target)
			}
		}
	}

	return nil
}

func (pm *PackageManager) GetPackageInfo(ctx context.Context, name string) (*PackageInfo, error) {
	if err := pm.updateDB(ctx, DefaultArch); err != nil {
		return nil, err
	}
	return pm.findPackage(name, "")
}

func (pm *PackageManager) SearchPackages(ctx context.Context, query string) ([]*PackageInfo, error) {
	if err := pm.updateDB(ctx, DefaultArch); err != nil {
		return nil, err
	}
	var results []*PackageInfo
	for _, p := range pm.cache.packages {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			results = append(results, p)
		}
	}
	return results, nil
}

// GetDependencies returns the list of dependencies for a package
func (pm *PackageManager) GetDependencies(ctx context.Context, name string) ([]Dependency, error) {
	if err := pm.updateDB(ctx, DefaultArch); err != nil {
		return nil, err
	}

	pkg, err := pm.findPackage(name, "")
	if err != nil {
		return nil, err
	}

	return pkg.Dependencies, nil
}