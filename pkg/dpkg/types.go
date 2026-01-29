// pkg/dpkg/types.go
package dpkg

import (
	"log"
	"time"
)

// Config configures the Debian package manager
type Config struct {
	RepositoryURL string        // Default: http://deb.debian.org/debian
	SecurityURL   string        // Security updates repository
	Release       string        // Debian release (bookworm, bullseye, etc.)
	Component     string        // Repository component (main, contrib, non-free)
	InstallPath   string        // Where to install packages
	CachePath     string        // Where to cache downloaded files
	Timeout       time.Duration
	Debug         bool        // Enable debug logging
	Logger        *log.Logger // Custom logger (optional)
}

// PackageManager handles Debian package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
	cache  *PackageCache
}

// PackageInfo contains metadata about a Debian package from Packages file
type PackageInfo struct {
	Package       string
	Version       string
	Architecture  string
	Maintainer    string
	InstalledSize int64
	Depends       []string
	Recommends    []string
	Suggests      []string
	Conflicts     []string
	Replaces      []string
	Provides      []string
	Description   string
	Homepage      string
	Section       string
	Priority      string
	Filename      string
	Size          int64
	MD5sum        string
	SHA1          string
	SHA256        string
	SHA512        string
}

// DownloadOptions configures package download and extraction
type DownloadOptions struct {
	Package      string       // Required: package name (e.g., "wget")
	Version      string       // Optional: specific version (uses latest if empty)
	Architecture Architecture // Target architecture (auto-detected if empty)
	Extract      bool         // Whether to extract the .deb (default: true)
	KeepArchive  bool         // Whether to keep the .deb after extraction (default: false)
	VerifyHash   bool         // Whether to verify SHA256 hash (default: true)
}

// PackageCache caches package index information
type PackageCache struct {
	packages      map[string]*PackageInfo // key: package_architecture
	lastUpdate    time.Time
	cacheDuration time.Duration
}

// Release represents a Debian release file
type Release struct {
	Origin        string
	Label         string
	Suite         string
	Version       string
	Codename      string
	Date          time.Time
	Architectures []string
	Components    []string
	Description   string
	MD5Sum        []FileHash
	SHA1          []FileHash
	SHA256        []FileHash
	SHA512        []FileHash
}

// FileHash represents a file hash entry in Release file
type FileHash struct {
	Hash string
	Size int64
	Name string
}