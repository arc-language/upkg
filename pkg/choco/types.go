// pkg/choco/types.go
package choco

import (
	"log"
	"time"
)

// Config configures the Chocolatey package manager
type Config struct {
	RepositoryURL string        // Default: https://community.chocolatey.org/api/v2
	InstallPath   string        // Where to extract packages
	CachePath     string        // Where to cache downloaded files
	Timeout       time.Duration
	Debug         bool
	Logger        *log.Logger
}

// PackageManager handles Chocolatey package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
}

// PackageInfo contains metadata about a Chocolatey package
type PackageInfo struct {
	ID              string   // Package ID (e.g., "curl")
	Version         string   // Version
	Title           string   // Display title
	Description     string   // Description
	Summary         string   // Short summary
	Authors         string   // Authors
	Owners          string   // Owners
	ProjectURL      string   // Project URL
	LicenseURL      string   // License URL
	IconURL         string   // Icon URL
	Tags            string   // Tags (space-separated)
	Dependencies    []string // Dependencies
	PackageHash     string   // Package hash
	PackageHashAlgo string   // Hash algorithm
	PackageSize     int64    // Package size in bytes
	Published       string   // Published date
	DownloadCount   int64    // Download count
}

// DownloadOptions configures package download and extraction
type DownloadOptions struct {
	Package     string // Required: package ID (e.g., "curl")
	Version     string // Optional: specific version (uses latest if empty)
	Extract     bool   // Whether to extract the .nupkg (default: true)
	KeepArchive bool   // Whether to keep the .nupkg after extraction (default: false)
	VerifyHash  bool   // Whether to verify checksum (default: true)
}