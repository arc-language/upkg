// pkg/backend/dpkg.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/dpkg"
)

// DpkgBackend implements the Backend interface for Debian packages
type DpkgBackend struct {
	manager *dpkg.PackageManager
	config  *Config
}

// NewDpkgBackend creates a new Debian package backend
func NewDpkgBackend(config *Config) (*DpkgBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	dpkgConfig := &dpkg.Config{
		RepositoryURL: "http://deb.debian.org/debian",
		SecurityURL:   "http://security.debian.org/debian-security",
		Release:       "bookworm",
		Component:     "main",
		InstallPath:   config.InstallPath,
		CachePath:     config.CachePath,
		Timeout:       config.Timeout,
		Debug:         config.Debug,
		Logger:        config.Logger,
	}

	manager := dpkg.NewPackageManager(dpkgConfig)

	return &DpkgBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using dpkg/Debian
func (b *DpkgBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	dpkgOpts := &dpkg.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		dpkgOpts.Architecture = dpkg.Architecture(opts.Platform)
	}

	return b.manager.Download(ctx, dpkgOpts)
}

// GetInfo retrieves package information from Debian
func (b *DpkgBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	arch, err := dpkg.DetectArchitecture()
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
		Homepage:    pkgInfo.Homepage,
		License:     "", // Debian packages don't include license in Packages file
		Platforms:   []string{pkgInfo.Architecture},
		Backend:     "dpkg",
	}, nil
}

// Search searches for packages in Debian repositories
func (b *DpkgBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	arch, err := dpkg.DetectArchitecture()
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
			Homepage:    pkg.Homepage,
			Platforms:   []string{pkg.Architecture},
			Backend:     "dpkg",
		})
	}

	return results, nil
}

// Name returns the backend name
func (b *DpkgBackend) Name() string {
	return "dpkg"
}

// Close cleans up resources
func (b *DpkgBackend) Close() error {
	return nil
}