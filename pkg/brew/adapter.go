// pkg/backends/brew/adapter.go
package brew

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/core"
	brewpkg "github.com/arc-language/upkg/pkg/brew"
)

// Adapter adapts the brew package manager to the core interface
type Adapter struct {
	pm *brewpkg.PackageManager
}

// NewAdapter creates a new Brew adapter
func NewAdapter(installPath string, debug bool) *Adapter {
	cfg := &brewpkg.Config{
		InstallPath: installPath,
		Debug:       debug,
	}
	
	return &Adapter{
		pm: brewpkg.NewPackageManager(cfg),
	}
}

// Name returns the backend name
func (a *Adapter) Name() string {
	return "brew"
}

// Install installs a package
func (a *Adapter) Install(ctx context.Context, pkg string, opts *core.InstallOptions) error {
	if opts == nil {
		opts = &core.InstallOptions{
			Extract:    true,
			VerifyHash: true,
		}
	}

	brewOpts := &brewpkg.DownloadOptions{
		Formula:     pkg,
		Version:     opts.Version,
		Platform:    brewpkg.Platform(opts.Platform),
		Extract:     opts.Extract,
		KeepArchive: opts.KeepArchive,
		VerifyHash:  opts.VerifyHash,
	}

	return a.pm.Download(ctx, brewOpts)
}

// Remove removes a package (not supported by Brew downloader)
func (a *Adapter) Remove(ctx context.Context, pkg string) error {
	return fmt.Errorf("remove not supported by brew backend")
}

// Search searches for packages (not supported by Brew downloader)
func (a *Adapter) Search(ctx context.Context, query string) ([]core.Package, error) {
	return nil, fmt.Errorf("search not supported by brew backend")
}

// List lists installed packages (not supported by Brew downloader)
func (a *Adapter) List(ctx context.Context) ([]core.Package, error) {
	return nil, fmt.Errorf("list not supported by brew backend")
}

// Info gets package information
func (a *Adapter) Info(ctx context.Context, pkg string) (*core.Package, error) {
	formula, err := a.pm.GetFormulaInfo(ctx, pkg)
	if err != nil {
		return nil, err
	}

	platform, _ := brewpkg.DetectPlatform()

	return &core.Package{
		Name:        formula.Name,
		Version:     formula.Versions.Stable,
		Description: formula.Description,
		Backend:     "brew",
		Platform:    string(platform),
		Installed:   false,
	}, nil
}

// IsAvailable checks if Homebrew is available
func (a *Adapter) IsAvailable() bool {
	_, err := brewpkg.DetectPlatform()
	return err == nil
}

// Update updates package lists (not supported by Brew downloader)
func (a *Adapter) Update(ctx context.Context) error {
	return fmt.Errorf("update not supported by brew backend")
}