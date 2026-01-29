// pkg/backend/types.go
package backend

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// BackendType represents the package manager backend
type BackendType string

const (
	// BackendNix uses the Nix package manager
	BackendNix BackendType = "nix"
	// BackendBrew uses the Homebrew package manager
	BackendBrew BackendType = "brew"
	// BackendDpkg uses the Debian package manager
	BackendDpkg BackendType = "dpkg"
	// BackendApt uses the Ubuntu package manager
	BackendApt BackendType = "apt"
	// BackendApk uses the Alpine package manager
	BackendApk BackendType = "apk"
	// BackendDnf uses the Fedora package manager
	BackendDnf BackendType = "dnf"
	// BackendAuto automatically detects the best backend
	BackendAuto BackendType = "auto"
)

// Backend defines the interface that all package manager backends must implement
type Backend interface {
	// Download downloads and installs a package
	Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error

	// GetInfo retrieves information about a package
	GetInfo(ctx context.Context, name string) (*PackageInfo, error)

	// Search searches for packages
	Search(ctx context.Context, query string) ([]*PackageInfo, error)

	// Name returns the name of the backend
	Name() string

	// Close cleans up resources
	Close() error
}

// Package represents a package to download
type Package struct {
	Name    string // Package name (e.g., "wget", "gcc")
	Version string // Optional: specific version
	Output  string // Optional: for Nix, which output to use (bin, dev, lib, etc.)
	Hash    string // Optional: for Nix, specific store hash
}

// PackageInfo contains metadata about a package
type PackageInfo struct {
	Name        string            // Package name
	Version     string            // Package version
	Description string            // Package description
	Homepage    string            // Project homepage
	License     string            // License information
	Platforms   []string          // Supported platforms
	Outputs     map[string]string // For Nix: output names to hashes
	Backend     string            // Which backend this came from
}

// DownloadOptions configures package download behavior
type DownloadOptions struct {
	Platform    string // Target platform (auto-detected if empty)
	Extract     *bool  // Whether to extract archives (default: true)
	KeepArchive *bool  // Whether to keep downloaded archives (default: false)
	VerifyHash  *bool  // Whether to verify checksums (default: true)
	Force       bool   // Force re-download even if exists
}

// Config holds configuration for the universal package manager
type Config struct {
	// InstallPath is where packages will be installed
	InstallPath string

	// CachePath is where downloaded files are cached
	CachePath string

	// Timeout for network operations
	Timeout time.Duration

	// Debug enables debug logging
	Debug bool

	// Logger for custom logging
	Logger *log.Logger

	// Nix-specific configuration
	Nix *NixConfig

	// Brew-specific configuration
	Brew *BrewConfig
}

// NixConfig holds Nix-specific configuration
type NixConfig struct {
	CacheURL string // Default: https://cache.nixos.org
}

// BrewConfig holds Homebrew-specific configuration
type BrewConfig struct {
	APIURL      string // Default: https://formulae.brew.sh/api
	RegistryURL string // Default: https://ghcr.io/v2/homebrew/core
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()

	installPath := filepath.Join(homeDir, ".upkg")
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		installPath = "/opt/homebrew" // Use Homebrew default on ARM Mac
	}

	return &Config{
		InstallPath: installPath,
		CachePath:   filepath.Join(homeDir, ".cache", "upkg"),
		Timeout:     2 * time.Minute,
		Debug:       false,
		Nix: &NixConfig{
			CacheURL: "https://cache.nixos.org",
		},
		Brew: &BrewConfig{
			APIURL:      "https://formulae.brew.sh/api",
			RegistryURL: "https://ghcr.io/v2/homebrew/core",
		},
	}
}

// derefBool dereferences a bool pointer with a default value
func derefBool(ptr *bool, defaultVal bool) bool {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}