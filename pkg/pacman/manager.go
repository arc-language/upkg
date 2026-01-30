// pkg/pacman/manager.go
package pacman

import (
	"archive/tar"
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

	"github.com/klauspost/compress/zstd"
)

func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	if cfg.MirrorURL == "" {
		cfg.MirrorURL = DefaultMirror
	}
	if len(cfg.Repos) == 0 {
		cfg.Repos = DefaultRepos
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.CachePath == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.CachePath = filepath.Join(homeDir, ".cache", "upkg", "pacman")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[PACMAN] ", log.LstdFlags)
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
			providers:     make(map[string][]*PackageInfo),
			cacheDuration: 30 * time.Minute,
		},
	}
}

// Download performs the package download and installation
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts.Architecture == "" {
		opts.Architecture = DefaultArch
	}

	pm.logger.Printf("Starting operation for package: %s", opts.Package)

	// 1. Sync DB (once at start)
	if err := pm.updateDB(ctx, opts.Architecture); err != nil {
		return err
	}

	// Track installed packages to avoid loops
	visited := make(map[string]bool)

	// Start recursive installation
	return pm.installRecursive(ctx, opts.Package, visited, opts)
}

// installRecursive handles dependencies and installation
func (pm *PackageManager) installRecursive(ctx context.Context, pkgName string, visited map[string]bool, opts *DownloadOptions) error {
	// 1. Resolve Package (handle providers like "sh" -> "bash")
	pkg, err := pm.resolvePackage(pkgName)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", pkgName, err)
	}

	// Check if already visited using the REAL name
	if visited[pkg.Name] {
		return nil
	}
	visited[pkg.Name] = true

	pm.logger.Printf("Processing package: %s (repo: %s)", pkg.Name, pkg.Repository)

	// 2. Install Dependencies
	for _, depStr := range pkg.Depends {
		// Clean dependency string (e.g. "glibc>=2.35" -> "glibc")
		depName := cleanDepName(depStr)
		
		// Skip self-references or cycles if possible
		if depName == pkg.Name { continue }

		pm.logger.Printf("  -> Dependency: %s", depName)
		
		if err := pm.installRecursive(ctx, depName, visited, opts); err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to install dependency %s: %v", depName, err)
		}
	}

	// 3. Download
	// URL format: https://mirror/repo/os/arch/filename
	// Fallback filename if empty in DB (sometimes happens with cached DBs)
	filename := pkg.Filename
	if filename == "" {
		filename = fmt.Sprintf("%s-%s-%s.pkg.tar.zst", pkg.Name, pkg.Version, pkg.Architecture)
	}

	downloadURL := fmt.Sprintf("%s/%s/os/%s/%s", 
		pm.config.MirrorURL, pkg.Repository, pkg.Architecture, filename)
	
	destPath := filepath.Join(pm.config.CachePath, "downloads", filename)

	pm.logger.Printf("  Downloading %s...", pkg.Name)
	if err := pm.downloadFile(ctx, downloadURL, destPath); err != nil {
		return fmt.Errorf("downloading %s: %w", pkg.Name, err)
	}

	// 4. Verify
	if opts.VerifyHash && pkg.SHA256Sum != "" {
		if err := pm.verifyHash(destPath, pkg.SHA256Sum); err != nil {
			return fmt.Errorf("hash verification failed for %s: %w", pkg.Name, err)
		}
	}

	// 5. Extract
	if opts.Extract {
		pm.logger.Printf("  Extracting %s...", pkg.Name)
		if err := pm.extractZstdPackage(destPath, pm.config.InstallPath); err != nil {
			return fmt.Errorf("extracting %s: %w", pkg.Name, err)
		}
		if !opts.KeepArchive {
			os.Remove(destPath)
		}
	}

	pm.logger.Printf("✓ Installed %s", pkg.Name)
	return nil
}

// updateDB downloads and indexes repositories
func (pm *PackageManager) updateDB(ctx context.Context, arch string) error {
	if len(pm.cache.packages) > 0 && time.Since(pm.cache.lastUpdate) < pm.cache.cacheDuration {
		return nil
	}

	pm.logger.Printf("Syncing databases...")
	
	// Reset caches
	pm.cache.packages = make(map[string]*PackageInfo)
	pm.cache.providers = make(map[string][]*PackageInfo)

	for _, repo := range pm.config.Repos {
		// DB URL: https://mirror/repo/os/arch/repo.db
		url := fmt.Sprintf("%s/%s/os/%s/%s.db", pm.config.MirrorURL, repo, arch, repo)
		
		pm.logger.Printf("  Fetching %s.db", repo)
		body, err := pm.client.Get(ctx, url)
		if err != nil {
			pm.logger.Printf("    ⚠️ Failed to fetch %s: %v", repo, err)
			continue
		}

		pkgs, err := ParseDatabase(body, repo)
		body.Close()
		if err != nil {
			pm.logger.Printf("    ⚠️ Failed to parse %s: %v", repo, err)
			continue
		}

		// Index packages
		for _, p := range pkgs {
			// 1. Name map
			pm.cache.packages[p.Name] = p
			
			// 2. Provider map (Real name is a provider of itself)
			pm.cache.providers[p.Name] = append(pm.cache.providers[p.Name], p)
			
			// 3. Virtual providers (e.g. bash provides "sh")
			for _, prov := range p.Provides {
				cleanProv := cleanDepName(prov)
				pm.cache.providers[cleanProv] = append(pm.cache.providers[cleanProv], p)
			}
		}
		pm.logger.Printf("    Indexed %d packages from %s", len(pkgs), repo)
	}

	pm.cache.lastUpdate = time.Now()
	return nil
}

// resolvePackage finds a package by name or virtual provider
func (pm *PackageManager) resolvePackage(name string) (*PackageInfo, error) {
	cleanName := cleanDepName(name)

	// 1. Check direct package name
	if pkg, ok := pm.cache.packages[cleanName]; ok {
		return pkg, nil
	}

	// 2. Check providers
	if providers, ok := pm.cache.providers[cleanName]; ok && len(providers) > 0 {
		// Simple heuristic: Pick first one
		// In reality, we might prefer "core" repo over "extra"
		return providers[0], nil
	}

	return nil, fmt.Errorf("package %s not found", name)
}

// findPackage is a public helper for single lookups (used by GetInfo)
func (pm *PackageManager) findPackage(name, version string) (*PackageInfo, error) {
	return pm.resolvePackage(name)
}

// Helper to remove version constraints (e.g., "glibc>=2.35" -> "glibc")
func cleanDepName(dep string) string {
	if idx := strings.IndexAny(dep, "><="); idx != -1 {
		return dep[:idx]
	}
	return dep
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

func (pm *PackageManager) verifyHash(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("hash mismatch: want %s, got %s", expected, actual)
	}
	return nil
}

// extractZstdPackage extracts .pkg.tar.zst
func (pm *PackageManager) extractZstdPackage(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	// Zstd decoder
	zstdReader, err := zstd.NewReader(f)
	if err != nil {
		return fmt.Errorf("zstd init: %w", err)
	}
	defer zstdReader.Close()

	tarReader := tar.NewReader(zstdReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip metadata files (usually strictly at root level starting with dot)
		if strings.HasPrefix(header.Name, ".") {
			continue
		}

		target := filepath.Join(dest, header.Name)
		
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			// Remove existing if present to avoid error
			os.Remove(target)
			os.Symlink(header.Linkname, target)
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
	query = strings.ToLower(query)
	for _, p := range pm.cache.packages {
		if strings.Contains(strings.ToLower(p.Name), query) {
			results = append(results, p)
		}
	}
	return results, nil
}