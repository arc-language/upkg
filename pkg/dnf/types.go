// pkg/dnf/types.go
package dnf

import (
	"log"
	"time"
)

// Config configures the Fedora/DNF package manager
type Config struct {
	RepositoryURL string        // Default: https://dl.fedoraproject.org/pub/fedora/linux
	Release       string        // Fedora release (42, 41, etc.)
	Repository    string        // Repository name (releases, updates, etc.)
	InstallPath   string        // Where to install packages
	CachePath     string        // Where to cache downloaded files
	Timeout       time.Duration
	Debug         bool        // Enable debug logging
	Logger        *log.Logger // Custom logger (optional)
}

// PackageManager handles Fedora/DNF package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
	cache  *PackageCache
}

// PackageInfo contains metadata about a Fedora package from repodata
type PackageInfo struct {
	Name         string   // Package name
	Version      string   // Version
	Release      string   // Release
	Epoch        string   // Epoch
	Architecture string   // Architecture (x86_64, aarch64, etc.)
	Summary      string   // Short description
	Description  string   // Long description
	URL          string   // Project URL
	License      string   // License
	Vendor       string   // Vendor
	Packager     string   // Packager
	Size         int64    // Package size
	InstalledSize int64   // Installed size
	Location     string   // File location in repository
	Checksum     string   // SHA256 checksum
	ChecksumType string   // Checksum type (sha256, sha512, etc.)
	Requires     []string // Dependencies
	Provides     []string // Provides
	Conflicts    []string // Conflicts
	Obsoletes    []string // Obsoletes
}

// DownloadOptions configures package download and extraction
type DownloadOptions struct {
	Package      string       // Required: package name (e.g., "curl")
	Version      string       // Optional: specific version (uses latest if empty)
	Architecture Architecture // Target architecture (auto-detected if empty)
	Extract      bool         // Whether to extract the .rpm (default: true)
	KeepArchive  bool         // Whether to keep the .rpm after extraction (default: false)
	VerifyHash   bool         // Whether to verify checksum (default: true)
}

// PackageCache caches package index information
type PackageCache struct {
	packages      map[string]*PackageInfo   // key: package_name
	providers     map[string][]*PackageInfo // key: virtual_provide -> list of packages
	lastUpdate    time.Time
	cacheDuration time.Duration
}

// RepoMD represents the repomd.xml file structure
type RepoMD struct {
	Revision string
	Data     []RepoData
}

// RepoData represents a data entry in repomd.xml
type RepoData struct {
	Type         string
	Location     string
	Checksum     string
	OpenChecksum string
	Timestamp    int64
	Size         int64
	OpenSize     int64
}