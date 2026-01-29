// pkg/backend/apk.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/apk"
)

// ApkBackend implements the Backend interface for Alpine packages
type ApkBackend struct {
	manager *apk.PackageManager
	config  *Config
}

// NewApkBackend creates a new Alpine APK backend
func NewApkBackend(config *Config) (*ApkBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	apkConfig := &apk.Config{
		RepositoryURL: "https://dl-cdn.alpinelinux.org/alpine",
		Branch:        "v3.19",
		Repository:    "main",
		InstallPath:   config.InstallPath,
		CachePath:     config.CachePath,
		Timeout:       config.Timeout,
		Debug:         config.Debug,
		Logger:        config.Logger,
	}

	manager := apk.NewPackageManager(apkConfig)

	return &ApkBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using APK/Alpine
func (b *ApkBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	apkOpts := &apk.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		apkOpts.Architecture = apk.Architecture(opts.Platform)
	}

	return b.manager.Download(ctx, apkOpts)
}

// GetInfo retrieves package information from Alpine
func (b *ApkBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	arch, err := apk.DetectArchitecture()
	if err != nil {
		return nil, fmt.Errorf("detecting architecture: %w", err)
	}

	pkgInfo, err := b.manager.GetPackageInfo(ctx, name, arch)
	if err != nil {
		return nil, fmt.Errorf("getting package info: %w", err)
	}

	return &PackageInfo{
		Name:        pkgInfo.Package,
		Version:     pkgInfo.Version,
		Description: pkgInfo.Description,
		Homepage:    pkgInfo.URL,
		License:     pkgInfo.License,
		Platforms:   []string{pkgInfo.Architecture},
		Backend:     "apk",
	}, nil
}

// Search searches for packages in Alpine repositories
func (b *ApkBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	arch, err := apk.DetectArchitecture()
	if err != nil {
		return nil, fmt.Errorf("detecting architecture: %w", err)
	}

	packages, err := b.manager.SearchPackages(ctx, query, arch)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}

	results := make([]*PackageInfo, 0, len(packages))
	for _, pkg := range packages {
		results = append(results, &PackageInfo{
			Name:        pkg.Package,
			Version:     pkg.Version,
			Description: pkg.Description,
			Homepage:    pkg.URL,
			License:     pkg.License,
			Platforms:   []string{pkg.Architecture},
			Backend:     "apk",
		})
	}

	return results, nil
}

// Name returns the backend name
func (b *ApkBackend) Name() string {
	return "apk"
}

// Close cleans up resources
func (b *ApkBackend) Close() error {
	return nil
}