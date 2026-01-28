// upkg.go
package upkg

import (
	"context"
	"fmt"
	"runtime"

	"github.com/arc-language/upkg/pkg/backend"
)

// Manager is the universal package manager
type Manager struct {
	backend backend.Backend
	config  *backend.Config
}

// NewManager creates a new universal package manager with the specified backend
func NewManager(backendType backend.BackendType, config *backend.Config) (*Manager, error) {
	if config == nil {
		config = backend.DefaultConfig()
	}

	var b backend.Backend
	var err error

	switch backendType {
	case backend.BackendNix:
		b, err = backend.NewNixBackend(config)
	case backend.BackendBrew:
		b, err = backend.NewBrewBackend(config)
	case backend.BackendAuto:
		b, err = autoDetectBackend(config)
	default:
		return nil, fmt.Errorf("unsupported backend type: %s", backendType)
	}

	if err != nil {
		return nil, fmt.Errorf("initializing backend: %w", err)
	}

	return &Manager{
		backend: b,
		config:  config,
	}, nil
}

// autoDetectBackend automatically selects the best backend for the current system
func autoDetectBackend(config *backend.Config) (backend.Backend, error) {
	// Check if Homebrew is available (macOS or Linux with Homebrew)
	if runtime.GOOS == "darwin" {
		// Prefer Homebrew on macOS
		b, err := backend.NewBrewBackend(config)
		if err == nil {
			return b, nil
		}
	}

	// Try Nix as fallback
	b, err := backend.NewNixBackend(config)
	if err == nil {
		return b, nil
	}

	// If on Linux, try Homebrew as last resort
	if runtime.GOOS == "linux" {
		b, err := backend.NewBrewBackend(config)
		if err == nil {
			return b, nil
		}
	}

	return nil, fmt.Errorf("no suitable package manager backend found")
}

// Download downloads and installs a package
func (m *Manager) Download(ctx context.Context, pkg *backend.Package, opts *backend.DownloadOptions) error {
	if pkg == nil {
		return fmt.Errorf("package cannot be nil")
	}
	if pkg.Name == "" {
		return fmt.Errorf("package name is required")
	}

	if opts == nil {
		opts = &backend.DownloadOptions{}
	}

	// Set defaults
	if opts.Extract == nil {
		extract := true
		opts.Extract = &extract
	}
	if opts.VerifyHash == nil {
		verify := true
		opts.VerifyHash = &verify
	}

	return m.backend.Download(ctx, pkg, opts)
}

// GetInfo retrieves information about a package
func (m *Manager) GetInfo(ctx context.Context, name string) (*backend.PackageInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	return m.backend.GetInfo(ctx, name)
}

// Search searches for packages by name or keyword
func (m *Manager) Search(ctx context.Context, query string) ([]*backend.PackageInfo, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	return m.backend.Search(ctx, query)
}

// Backend returns the name of the active backend
func (m *Manager) Backend() string {
	return m.backend.Name()
}

// Close cleans up any resources used by the manager
func (m *Manager) Close() error {
	return m.backend.Close()
}