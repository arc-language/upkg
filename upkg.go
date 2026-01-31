// upkg.go
package upkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/arc-language/upkg/pkg/backend"
	"github.com/arc-language/upkg/pkg/choco"
	"github.com/arc-language/upkg/pkg/index"
	"github.com/arc-language/upkg/pkg/registry"
)

// Re-export backend types for convenience
type (
	BackendType     = backend.BackendType
	Package         = backend.Package
	PackageInfo     = backend.PackageInfo
	DownloadOptions = backend.DownloadOptions
	Config          = backend.Config
	NixConfig       = backend.NixConfig
	BrewConfig      = backend.BrewConfig
	// RegistryEntry is the metadata for a package from the deps/ registry.
	// Re-exported so external tools like a compiler can access it.
	RegistryEntry = registry.Entry
)

// Re-export backend constants
const (
	BackendNix    = backend.BackendNix
	BackendBrew   = backend.BackendBrew
	BackendDpkg   = backend.BackendDpkg
	BackendApt    = backend.BackendApt
	BackendApk    = backend.BackendApk
	BackendDnf    = backend.BackendDnf
	BackendChoco  = backend.BackendChoco
	BackendPacman = backend.BackendPacman
	BackendZypper = backend.BackendZypper
	BackendWinget = backend.BackendWinget
	BackendAuto   = backend.BackendAuto
)

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return backend.DefaultConfig()
}

// Manager is the universal package manager
type Manager struct {
	backend  backend.Backend
	config   *backend.Config
	registry *registry.Registry // only set in auto mode
}

// NewManager creates a new universal package manager with the specified backend
func NewManager(backendType backend.BackendType, config *backend.Config) (*Manager, error) {
	if config == nil {
		config = backend.DefaultConfig()
	}

	// Ensure CachePath is set
	if config.CachePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			config.CachePath = filepath.Join(os.TempDir(), "upkg")
		} else {
			config.CachePath = filepath.Join(home, ".cache", "upkg")
		}
	}

	// Sync if deps folder doesn't exist yet
	depsDir := filepath.Join(config.CachePath, "deps")
	if _, err := os.Stat(depsDir); os.IsNotExist(err) {
		if err := index.Sync(config.CachePath); err != nil {
			return nil, fmt.Errorf("failed to sync package index: %w", err)
		}
	}

	var b backend.Backend
	var reg *registry.Registry
	var err error

	switch backendType {
	case backend.BackendNix:
		b, err = backend.NewNixBackend(config)
	case backend.BackendBrew:
		b, err = backend.NewBrewBackend(config)
	case backend.BackendDpkg:
		b, err = backend.NewDpkgBackend(config)
	case backend.BackendApt:
		b, err = backend.NewAptBackend(config)
	case backend.BackendApk:
		b, err = backend.NewApkBackend(config)
	case backend.BackendDnf:
		b, err = backend.NewDnfBackend(config)
	case backend.BackendChoco:
		b, err = backend.NewChocoBackend(config)
	case backend.BackendPacman:
		b, err = backend.NewPacmanBackend(config)
	case backend.BackendZypper:
		b, err = backend.NewZypperBackend(config)
	case backend.BackendWinget:
		b, err = backend.NewWingetBackend(config)
	case backend.BackendAuto:
		b, err = autoDetectBackend(config)
		if err == nil {
			reg = registry.New(config.CachePath)
		}
	default:
		return nil, fmt.Errorf("unsupported backend type: %s", backendType)
	}

	if err != nil {
		return nil, fmt.Errorf("initializing backend: %w", err)
	}

	return &Manager{
		backend:  b,
		config:   config,
		registry: reg,
	}, nil
}

// autoDetectBackend detects the best backend for the current system
func autoDetectBackend(config *backend.Config) (backend.Backend, error) {
	if runtime.GOOS == "darwin" {
		b, err := backend.NewBrewBackend(config)
		if err == nil {
			return b, nil
		}
	}

	if runtime.GOOS == "windows" {
		b, err := backend.NewWingetBackend(config)
		if err == nil {
			return b, nil
		}

		if err := choco.DetectPlatform(); err == nil {
			b, err := backend.NewChocoBackend(config)
			if err == nil {
				return b, nil
			}
		}
	}

	if runtime.GOOS == "linux" {
		if isAlpine() {
			b, err := backend.NewApkBackend(config)
			if err == nil {
				return b, nil
			}
		}

		if isFedora() {
			b, err := backend.NewDnfBackend(config)
			if err == nil {
				return b, nil
			}
		}

		if isArchLinux() {
			b, err := backend.NewPacmanBackend(config)
			if err == nil {
				return b, nil
			}
		}

		if isOpenSUSE() {
			b, err := backend.NewZypperBackend(config)
			if err == nil {
				return b, nil
			}
		}

		if isUbuntu() {
			b, err := backend.NewAptBackend(config)
			if err == nil {
				return b, nil
			}
		}

		b, err := backend.NewDpkgBackend(config)
		if err == nil {
			return b, nil
		}
	}

	b, err := backend.NewNixBackend(config)
	if err == nil {
		return b, nil
	}

	if runtime.GOOS == "linux" {
		b, err := backend.NewBrewBackend(config)
		if err == nil {
			return b, nil
		}
	}

	return nil, fmt.Errorf("no suitable package manager backend found")
}

func isFedora() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		if _, err := os.Stat("/etc/fedora-release"); err == nil {
			return true
		}
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "fedora") || strings.Contains(content, "rhel") || strings.Contains(content, "centos")
}

func isAlpine() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		if _, err := os.Stat("/etc/alpine-release"); err == nil {
			return true
		}
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "alpine")
}

func isArchLinux() bool {
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return true
	}

	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "arch") || strings.Contains(content, "manjaro")
}

func isOpenSUSE() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		if _, err := os.Stat("/etc/SuSE-release"); err == nil {
			return true
		}
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "opensuse") || strings.Contains(content, "sles")
}

func isUbuntu() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "ubuntu") && !strings.Contains(content, "debian")
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

	if opts.Extract == nil {
		extract := true
		opts.Extract = &extract
	}
	if opts.VerifyHash == nil {
		verify := true
		opts.VerifyHash = &verify
	}

	// In auto mode, resolve through registry before delegating
	resolvedPkg := *pkg
	if m.registry != nil {
		resolved, err := m.registry.Resolve(pkg.Name, m.backend.Name())
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if m.config.Debug && m.config.Logger != nil {
			m.config.Logger.Printf("Resolved '%s' -> '%s' (%s)", pkg.Name, resolved, m.backend.Name())
		}
		resolvedPkg.Name = resolved
	}

	return m.backend.Download(ctx, &resolvedPkg, opts)
}

// GetInfo retrieves information about a package
func (m *Manager) GetInfo(ctx context.Context, name string) (*backend.PackageInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("package name is required")
	}

	// In auto mode, try registry first, fall back to raw name
	resolved := name
	if m.registry != nil {
		if r, err := m.registry.Resolve(name, m.backend.Name()); err == nil {
			resolved = r
		}
	}

	return m.backend.GetInfo(ctx, resolved)
}

// Search searches for packages by name or keyword
func (m *Manager) Search(ctx context.Context, query string) ([]*backend.PackageInfo, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	return m.backend.Search(ctx, query)
}

// GetRegistryEntry retrieves the full registry entry for a package.
// This is useful for accessing metadata like the 'libs' field.
// Returns an error if not in auto mode or if the package is not found.
func (m *Manager) GetRegistryEntry(name string) (*RegistryEntry, error) {
	if m.registry == nil {
		return nil, fmt.Errorf("registry is only available in auto mode")
	}
	return m.registry.Load(name)
}

// Backend returns the name of the active backend
func (m *Manager) Backend() string {
	return m.backend.Name()
}

// Close cleans up any resources used by the manager
func (m *Manager) Close() error {
	return m.backend.Close()
}