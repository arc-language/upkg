// pkg/backend/apt.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/apt"
)

// AptBackend implements the Backend interface for Ubuntu packages
type AptBackend struct {
	manager *apt.PackageManager
	config  *Config
}

// NewAptBackend creates a new Ubuntu APT backend
func NewAptBackend(config *Config) (*AptBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	aptConfig := &apt.Config{
		RepositoryURL: "http://archive.ubuntu.com/ubuntu",
		SecurityURL:   "http://security.ubuntu.com/ubuntu",
		PortsURL:      "http://ports.ubuntu.com/ubuntu-ports",
		Release:       "noble", // Ubuntu 24.04 LTS
		Component:     "main",
		InstallPath:   config.InstallPath,
		CachePath:     config.CachePath,
		Timeout:       config.Timeout,
		Debug:         config.Debug,
		Logger:        config.Logger,
	}

	manager := apt.NewPackageManager(aptConfig)

	return &AptBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using APT/Ubuntu
func (b *AptBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	aptOpts := &apt.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		aptOpts.Architecture = apt.Architecture(opts.Platform)
	}

	return b.manager.Download(ctx, aptOpts)
}

// GetInfo retrieves package information from Ubuntu
func (b *AptBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	arch, err := apt.DetectArchitecture()
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
		License:     "", // Ubuntu packages don't include license in Packages file
		Platforms:   []string{pkgInfo.Architecture},
		Backend:     "apt",
	}, nil
}

// Search searches for packages in Ubuntu repositories
func (b *AptBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	arch, err := apt.DetectArchitecture()
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
			Backend:     "apt",
		})
	}

	return results, nil
}

// Name returns the backend name
func (b *AptBackend) Name() string {
	return "apt"
}

// Close cleans up resources
func (b *AptBackend) Close() error {
	return nil
}