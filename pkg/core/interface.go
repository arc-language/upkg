// pkg/core/interface.go
package core

import "context"

// PackageManager defines the common interface for all package manager backends
type PackageManager interface {
	// Name returns the backend name (e.g., "nix", "brew")
	Name() string

	// Install installs a package
	Install(ctx context.Context, pkg string, opts *InstallOptions) error

	// Remove removes a package (not all backends support this)
	Remove(ctx context.Context, pkg string) error

	// Search searches for packages (not all backends support this)
	Search(ctx context.Context, query string) ([]Package, error)

	// List lists installed packages (not all backends support this)
	List(ctx context.Context) ([]Package, error)

	// Info gets package information
	Info(ctx context.Context, pkg string) (*Package, error)

	// IsAvailable checks if this backend is available on the system
	IsAvailable() bool

	// Update updates package lists (not all backends support this)
	Update(ctx context.Context) error
}

// InstallOptions configures package installation
type InstallOptions struct {
	Version     string // Specific version to install
	Extract     bool   // Whether to extract archives
	KeepArchive bool   // Whether to keep downloaded archives
	VerifyHash  bool   // Whether to verify checksums
	Platform    string // Target platform (auto-detected if empty)
}