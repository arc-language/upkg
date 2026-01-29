// pkg/backend/pacman.go
package backend

import (
	"context"
	"fmt"
	"strings"

	"github.com/arc-language/upkg/pkg/pacman"
)

// PacmanBackend implements the Backend interface for Arch Linux packages
type PacmanBackend struct {
	manager *pacman.PackageManager
	config  *Config
}

// NewPacmanBackend creates a new Pacman backend
func NewPacmanBackend(config *Config) (*PacmanBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	pacmanConfig := &pacman.Config{
		// In a real scenario, this might come from a config file
		MirrorURL:   "https://geo.mirror.pkgbuild.com", 
		Repos:       []string{"core", "extra"},
		InstallPath: config.InstallPath,
		CachePath:   config.CachePath,
		Timeout:     config.Timeout,
		Debug:       config.Debug,
		Logger:      config.Logger,
	}

	manager := pacman.NewPackageManager(pacmanConfig)

	return &PacmanBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using Pacman
func (b *PacmanBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	pacOpts := &pacman.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		pacOpts.Architecture = opts.Platform
	}

	return b.manager.Download(ctx, pacOpts)
}

// GetInfo retrieves package information from Pacman
func (b *PacmanBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	pkgInfo, err := b.manager.GetPackageInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting package info: %w", err)
	}

	return &PackageInfo{
		Name:        pkgInfo.Name,
		Version:     pkgInfo.Version,
		Description: pkgInfo.Description,
		Homepage:    pkgInfo.URL,
		License:     strings.Join(pkgInfo.License, ", "),
		Platforms:   []string{pkgInfo.Architecture},
		Backend:     "pacman",
	}, nil
}

// Search searches for packages in Pacman repositories
func (b *PacmanBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	packages, err := b.manager.SearchPackages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}

	results := make([]*PackageInfo, 0, len(packages))
	for _, pkg := range packages {
		results = append(results, &PackageInfo{
			Name:        pkg.Name,
			Version:     pkg.Version,
			Description: pkg.Description,
			Homepage:    pkg.URL,
			Platforms:   []string{pkg.Architecture},
			Backend:     "pacman",
		})
	}

	return results, nil
}

// Name returns the backend name
func (b *PacmanBackend) Name() string {
	return "pacman"
}

// Close cleans up resources
func (b *PacmanBackend) Close() error {
	return nil
}