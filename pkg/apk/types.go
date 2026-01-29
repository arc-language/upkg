// pkg/apk/types.go
package apk

import (
	"log"
	"time"
)

// Config configures the Alpine package manager
type Config struct {
	RepositoryURL string        // Default: https://dl-cdn.alpinelinux.org/alpine
	Branch        string        // Alpine branch (v3.19, v3.18, edge, etc.)
	Repository    string        // Repository name (main, community, testing)
	InstallPath   string        // Where to install packages
	CachePath     string        // Where to cache downloaded files
	Timeout       time.Duration
	Debug         bool        // Enable debug logging
	Logger        *log.Logger // Custom logger (optional)
}

// PackageManager handles Alpine package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
	cache  *PackageCache
}

// PackageInfo contains metadata about an Alpine package from APKINDEX
type PackageInfo struct {
	Package       string   // Package name (P:)
	Version       string   // Package version (V:)
	Architecture  string   // Architecture (A:)
	PackageSize   int64    // Package size (S:)
	InstalledSize int64    // Installed size (I:)
	Description   string   // Description (T:)
	URL           string   // Project URL (U:)
	License       string   // License (L:)
	Origin        string   // Origin package (o:)
	Maintainer    string   // Maintainer (m:)
	BuildTime     int64    // Build timestamp (t:)
	Commit        string   // Git commit (c:)
	Depends       []string // Dependencies (D:)
	Provides      []string // Provides (p:)
	InstallIf     []string // Install if (i:)
	Checksum      string   // SHA256 checksum (C:)
}

// DownloadOptions configures package download and extraction
type DownloadOptions struct {
	Package      string       // Required: package name (e.g., "curl")
	Version      string       // Optional: specific version (uses latest if empty)
	Architecture Architecture // Target architecture (auto-detected if empty)
	Extract      bool         // Whether to extract the .apk (default: true)
	KeepArchive  bool         // Whether to keep the .apk after extraction (default: false)
	VerifyHash   bool         // Whether to verify SHA256 hash (default: true)
}

// PackageCache caches package index information
type PackageCache struct {
	packages      map[string]*PackageInfo // key: package_architecture_repo
	lastUpdate    time.Time
	cacheDuration time.Duration
}

// APKIndex represents the parsed APKINDEX
type APKIndex struct {
	Packages []*PackageInfo
}