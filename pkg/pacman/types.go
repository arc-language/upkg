package pacman

import (
	"log"
	"time"
)

// Config configures the Pacman package manager
type Config struct {
	MirrorURL   string        // Mirror URL (e.g., https://geo.mirror.pkgbuild.com)
	Repos       []string      // List of repositories to sync (core, extra)
	InstallPath string        // Where to install packages
	CachePath   string        // Where to cache downloaded files
	Timeout     time.Duration // Network timeout
	Debug       bool          // Enable debug logging
	Logger      *log.Logger   // Custom logger
}

// PackageManager handles Pacman package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
	cache  *PackageCache
}

// PackageInfo contains metadata from the 'desc' file in the sync db
type PackageInfo struct {
	Name           string
	Version        string
	Base           string
	Description    string
	URL            string
	Architecture   string
	BuildDate      int64
	InstallDate    int64
	Packager       string
	Size           int64  // Download size (CSIZE)
	InstalledSize  int64  // ISIZE
	Filename       string // The .pkg.tar.zst filename
	MD5Sum         string
	SHA256Sum      string
	PGPSignature   string
	License        []string
	Replaces       []string
	Groups         []string
	Depends        []string
	OptDepends     []string
	MakeDepends    []string
	CheckDepends   []string
	Conflicts      []string
	Provides       []string
	Repository     string // Which repo this came from (core, extra)
}

// DownloadOptions configures package download
type DownloadOptions struct {
	Package      string // Required
	Version      string // Optional
	Architecture string // Optional (defaults to x86_64)
	Extract      bool
	KeepArchive  bool
	VerifyHash   bool
}

// PackageCache caches package index information
type PackageCache struct {
	packages      map[string]*PackageInfo // key: package_name
	lastUpdate    time.Time
	cacheDuration time.Duration
}