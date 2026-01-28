// pkg/backend/nix.go
package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/nix"
)

// NixBackend implements the Backend interface for Nix
type NixBackend struct {
	manager *nix.PackageManager
	config  *Config
}

// NewNixBackend creates a new Nix backend
func NewNixBackend(config *Config) (*NixBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	nixConfig := &nix.Config{
		CacheURL:    config.Nix.CacheURL,
		InstallPath: config.InstallPath,
		Timeout:     config.Timeout,
		Debug:       config.Debug,
		Logger:      config.Logger,
	}

	if nixConfig.Logger == nil && config.Debug {
		nixConfig.Logger = config.Logger
	}

	manager := nix.NewPackageManager(nixConfig)

	return &NixBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using Nix
func (b *NixBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	nixOpts := &nix.DownloadOptions{
		StoreHash:   pkg.Hash,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		nixOpts.Platform = nix.Platform(opts.Platform)
	}

	return b.manager.Download(ctx, pkg.Name, pkg.Version, nixOpts)
}

// GetInfo retrieves package information from Nix
func (b *NixBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	platform, err := nix.DetectPlatform()
	if err != nil {
		return nil, fmt.Errorf("detecting platform: %w", err)
	}

	outputs, resolvedName, err := b.manager.ResolvePackageName(ctx, name, platform)
	if err != nil {
		return nil, fmt.Errorf("resolving package: %w", err)
	}

	return &PackageInfo{
		Name:        name,
		Version:     resolvedName,
		Description: "", // Nix doesn't provide this easily via Hydra
		Outputs:     outputs,
		Platforms:   []string{platform.String()},
		Backend:     "nix",
	}, nil
}

// Search is not implemented for Nix backend yet
func (b *NixBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	return nil, fmt.Errorf("search not implemented for Nix backend")
}

// Name returns the backend name
func (b *NixBackend) Name() string {
	return "nix"
}

// Close cleans up resources
func (b *NixBackend) Close() error {
	return nil
}