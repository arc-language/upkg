// upkg.go
package upkg

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/arc-language/upkg/pkg/backend"
	"github.com/arc-language/upkg/pkg/choco"
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
	BackendAuto   = backend.BackendAuto
)

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return backend.DefaultConfig()
}

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

// autoDetectBackend detects the best backend for the current system
func autoDetectBackend(config *backend.Config) (backend.Backend, error) {
	// Check if Homebrew is available (macOS or Linux with Homebrew)
	if runtime.GOOS == "darwin" {
		b, err := backend.NewBrewBackend(config)
		if err == nil {
			return b, nil
		}
	}

	if runtime.GOOS == "windows" {
		// Try Chocolatey on Windows
		if err := choco.DetectPlatform(); err == nil {
			b, err := backend.NewChocoBackend(config)
			if err == nil {
				return b, nil
			}
		}
	}

	if runtime.GOOS == "linux" {
		// Check if this is Alpine
		if isAlpine() {
			b, err := backend.NewApkBackend(config)
			if err == nil {
				return b, nil
			}
		}

		// Check if this is Fedora/RHEL
		if isFedora() {
			b, err := backend.NewDnfBackend(config)
			if err == nil {
				return b, nil
			}
		}

		// Check if this is Arch Linux
		if isArchLinux() {
			b, err := backend.NewPacmanBackend(config)
			if err == nil {
				return b, nil
			}
		}

		// Check if this is OpenSUSE
		if isOpenSUSE() {
			b, err := backend.NewZypperBackend(config)
			if err == nil {
				return b, nil
			}
		}

		// Check if this is Ubuntu
		if isUbuntu() {
			b, err := backend.NewAptBackend(config)
			if err == nil {
				return b, nil
			}
		}

		// Try dpkg for Debian
		b, err := backend.NewDpkgBackend(config)
		if err == nil {
			return b, nil
		}
	}

	// Try Nix as fallback (works on Linux and macOS)
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

// isFedora checks if the system is Fedora
func isFedora() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Also check for /etc/fedora-release
		if _, err := os.Stat("/etc/fedora-release"); err == nil {
			return true
		}
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "fedora") || strings.Contains(content, "rhel") || strings.Contains(content, "centos")
}

// isAlpine checks if the system is Alpine Linux
func isAlpine() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Also check for /etc/alpine-release
		if _, err := os.Stat("/etc/alpine-release"); err == nil {
			return true
		}
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "alpine")
}

// isArchLinux checks if the system is Arch Linux
func isArchLinux() bool {
	// Check for /etc/arch-release
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return true
	}

	// Check os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	// Covers Arch, Manjaro, EndeavourOS, etc.
	return strings.Contains(content, "arch") || strings.Contains(content, "manjaro")
}

// isOpenSUSE checks if the system is OpenSUSE
func isOpenSUSE() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Also check for /etc/SuSE-release
		if _, err := os.Stat("/etc/SuSE-release"); err == nil {
			return true
		}
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "opensuse") || strings.Contains(content, "sles")
}

// isUbuntu checks if the system is Ubuntu
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