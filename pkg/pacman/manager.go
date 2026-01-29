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

	// 1. Sync DB
	if err := pm.updateDB(ctx, opts.Architecture); err != nil {
		return err
	}

	// 2. Find
	pkg, err := pm.findPackage(opts.Package, opts.Version)
	if err != nil {
		return err
	}
	pm.logger.Printf("  Found package: %s %s (Repo: %s)", pkg.Name, pkg.Version, pkg.Repository)

	// 3. Download
	// URL format: https://mirror/repo/os/arch/filename
	// Note: Sometimes files are in subdirs, but usually at repo root on mirrors
	downloadURL := fmt.Sprintf("%s/%s/os/%s/%s", 
		pm.config.MirrorURL, pkg.Repository, pkg.Architecture, pkg.Filename)
	
	destPath := filepath.Join(pm.config.CachePath, "downloads", pkg.Filename)

	pm.logger.Printf("  Downloading from: %s", downloadURL)
	if err := pm.downloadFile(ctx, downloadURL, destPath); err != nil {
		return err
	}

	// 4. Verify
	if opts.VerifyHash && pkg.SHA256Sum != "" {
		if err := pm.verifyHash(destPath, pkg.SHA256Sum); err != nil {
			return err
		}
	}

	// 5. Extract
	if opts.Extract {
		pm.logger.Printf("  Extracting to: %s", pm.config.InstallPath)
		if err := pm.extractZstdPackage(destPath, pm.config.InstallPath); err != nil {
			return err
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

		for _, p := range pkgs {
			pm.cache.packages[p.Name] = p
		}
		pm.logger.Printf("    Indexed %d packages from %s", len(pkgs), repo)
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

		// Skip metadata files
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
	for _, p := range pm.cache.packages {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			results = append(results, p)
		}
	}
	return results, nil
}