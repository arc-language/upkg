// pkg/backends/nix/adapter.go
package nix

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/core"
	nixpkg "github.com/arc-language/upkg/pkg/nix"
)

// Adapter adapts the nix package manager to the core interface
type Adapter struct {
	pm *nixpkg.PackageManager
}

// NewAdapter creates a new Nix adapter
func NewAdapter(installPath string, debug bool) *Adapter {
	cfg := &nixpkg.Config{
		InstallPath: installPath,
		Debug:       debug,
	}
	
	return &Adapter{
		pm: nixpkg.NewPackageManager(cfg),
	}
}

// Name returns the backend name
func (a *Adapter) Name() string {
	return "nix"
}

// Install installs a package
func (a *Adapter) Install(ctx context.Context, pkg string, opts *core.InstallOptions) error {
	if opts == nil {
		opts = &core.InstallOptions{
			Extract:    true,
			VerifyHash: true,
		}
	}

	nixOpts := &nixpkg.DownloadOptions{
		Platform:    nixpkg.Platform(opts.Platform),
		Extract:     opts.Extract,
		KeepArchive: opts.KeepArchive,
		VerifyHash:  opts.VerifyHash,
	}

	return a.pm.Download(ctx, pkg, opts.Version, nixOpts)
}

// Remove removes a package (not supported by Nix downloader)
func (a *Adapter) Remove(ctx context.Context, pkg string) error {
	return fmt.Errorf("remove not supported by nix backend")
}

// Search searches for packages (not supported by Nix downloader)
func (a *Adapter) Search(ctx context.Context, query string) ([]core.Package, error) {
	return nil, fmt.Errorf("search not supported by nix backend")
}

// List lists installed packages (not supported by Nix downloader)
func (a *Adapter) List(ctx context.Context) ([]core.Package, error) {
	return nil, fmt.Errorf("list not supported by nix backend")
}

// Info gets package information
func (a *Adapter) Info(ctx context.Context, pkg string) (*core.Package, error) {
	// Detect platform
	platform, err := nixpkg.DetectPlatform()
	if err != nil {
		return nil, err
	}

	// Resolve package to get outputs
	outputs, nameVersion, err := a.pm.ResolvePackageName(ctx, pkg, platform)
	if err != nil {
		return nil, err
	}

	return &core.Package{
		Name:        pkg,
		Version:     nameVersion,
		Description: fmt.Sprintf("%d outputs available", len(outputs)),
		Backend:     "nix",
		Platform:    string(platform),
		Installed:   false,
	}, nil
}

// IsAvailable checks if Nix is available
func (a *Adapter) IsAvailable() bool {
	_, err := nixpkg.DetectPlatform()
	return err == nil
}

// Update updates package lists (not supported by Nix downloader)
func (a *Adapter) Update(ctx context.Context) error {
	return fmt.Errorf("update not supported by nix backend")
}