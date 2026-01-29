// pkg/backend/choco.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/choco"
)

// ChocoBackend implements the Backend interface for Chocolatey packages
type ChocoBackend struct {
	manager *choco.PackageManager
	config  *Config
}

// NewChocoBackend creates a new Chocolatey backend
func NewChocoBackend(config *Config) (*ChocoBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Check if we're on Windows
	if err := choco.DetectPlatform(); err != nil {
		return nil, err
	}

	chocoConfig := &choco.Config{
		RepositoryURL: "https://community.chocolatey.org/api/v2",
		InstallPath:   config.InstallPath,
		CachePath:     config.CachePath,
		Timeout:       config.Timeout,
		Debug:         config.Debug,
		Logger:        config.Logger,
	}

	manager := choco.NewPackageManager(chocoConfig)

	return &ChocoBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using Chocolatey
func (b *ChocoBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	chocoOpts := &choco.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	return b.manager.Download(ctx, chocoOpts)
}

// GetInfo retrieves package information from Chocolatey
func (b *ChocoBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	pkgInfo, err := b.manager.GetPackageInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting package info: %w", err)
	}

	return &PackageInfo{
		Name:        pkgInfo.ID,
		Version:     pkgInfo.Version,
		Description: pkgInfo.Description,
		Homepage:    pkgInfo.ProjectURL,
		License:     pkgInfo.LicenseURL,
		Platforms:   []string{"windows"},
		Backend:     "choco",
	}, nil
}

// Search searches for packages in Chocolatey repository
func (b *ChocoBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	packages, err := b.manager.SearchPackages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}

	results := make([]*PackageInfo, 0, len(packages))
	for _, pkg := range packages {
		results = append(results, &PackageInfo{
			Name:        pkg.ID,
			Version:     pkg.Version,
			Description: pkg.Description,
			Homepage:    pkg.ProjectURL,
			License:     pkg.LicenseURL,
			Platforms:   []string{"windows"},
			Backend:     "choco",
		})
	}

	return results, nil
}

// Name returns the backend name
func (b *ChocoBackend) Name() string {
	return "choco"
}

// Close cleans up resources
func (b *ChocoBackend) Close() error {
	return nil
}