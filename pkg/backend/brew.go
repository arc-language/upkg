// pkg/backend/brew.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/brew"
)

// BrewBackend implements the Backend interface for Homebrew
type BrewBackend struct {
	manager *brew.PackageManager
	config  *Config
}

// NewBrewBackend creates a new Homebrew backend
func NewBrewBackend(config *Config) (*BrewBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	brewConfig := &brew.Config{
		APIURL:      config.Brew.APIURL,
		RegistryURL: config.Brew.RegistryURL,
		InstallPath: config.InstallPath,
		Timeout:     config.Timeout,
		Debug:       config.Debug,
		Logger:      config.Logger,
	}

	manager := brew.NewPackageManager(brewConfig)

	return &BrewBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using Homebrew
func (b *BrewBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	brewOpts := &brew.DownloadOptions{
		Formula:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		brewOpts.Platform = brew.Platform(opts.Platform)
	}

	return b.manager.Download(ctx, brewOpts)
}

// GetInfo retrieves package information from Homebrew
func (b *BrewBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	formula, err := b.manager.GetFormulaInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting formula info: %w", err)
	}

	// Extract available platforms from bottle info
	platforms := make([]string, 0)
	for platform := range formula.Bottle.Stable.Files {
		platforms = append(platforms, platform)
	}

	return &PackageInfo{
		Name:        formula.Name,
		Version:     formula.Versions.Stable,
		Description: formula.Description,
		Homepage:    formula.Homepage,
		License:     formula.License,
		Platforms:   platforms,
		Backend:     "brew",
	}, nil
}

// Search searches for packages in Homebrew
func (b *BrewBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	// Homebrew search would require additional API endpoints
	return nil, fmt.Errorf("search not implemented for Homebrew backend")
}

// Name returns the backend name
func (b *BrewBackend) Name() string {
	return "brew"
}

// Close cleans up resources
func (b *BrewBackend) Close() error {
	return nil
}