// pkg/backend/dnf.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/dnf"
)

// DnfBackend implements the Backend interface for Fedora packages
type DnfBackend struct {
	manager *dnf.PackageManager
	config  *Config
}

// NewDnfBackend creates a new Fedora DNF backend
func NewDnfBackend(config *Config) (*DnfBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	dnfConfig := &dnf.Config{
		RepositoryURL: "https://dl.fedoraproject.org/pub/fedora/linux",
		Release:       "40",
		Repository:    "releases",
		InstallPath:   config.InstallPath,
		CachePath:     config.CachePath,
		Timeout:       config.Timeout,
		Debug:         config.Debug,
		Logger:        config.Logger,
	}

	manager := dnf.NewPackageManager(dnfConfig)

	return &DnfBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using DNF/Fedora
func (b *DnfBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	dnfOpts := &dnf.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		dnfOpts.Architecture = dnf.Architecture(opts.Platform)
	}

	return b.manager.Download(ctx, dnfOpts)
}

// GetInfo retrieves package information from Fedora
func (b *DnfBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	arch, err := dnf.DetectArchitecture()
	if err != nil {
		return nil, fmt.Errorf("detecting architecture: %w", err)
	}

	pkgInfo, err := b.manager.GetPackageInfo(ctx, name, arch)
	if err != nil {
		return nil, fmt.Errorf("getting package info: %w", err)
	}

	return &PackageInfo{
		Name:        pkgInfo.Name,
		Version:     pkgInfo.FullVersion(),
		Description: pkgInfo.Description,
		Homepage:    pkgInfo.URL,
		License:     pkgInfo.License,
		Platforms:   []string{pkgInfo.Architecture},
		Backend:     "dnf",
	}, nil
}

// Search searches for packages in Fedora repositories
func (b *DnfBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	arch, err := dnf.DetectArchitecture()
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
			Name:        pkg.Name,
			Version:     pkg.FullVersion(),
			Description: pkg.Description,
			Homepage:    pkg.URL,
			License:     pkg.License,
			Platforms:   []string{pkg.Architecture},
			Backend:     "dnf",
		})
	}

	return results, nil
}

// Name returns the backend name
func (b *DnfBackend) Name() string {
	return "dnf"
}

// Close cleans up resources
func (b *DnfBackend) Close() error {
	return nil
}