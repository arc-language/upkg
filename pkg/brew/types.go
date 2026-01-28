// types.go
package brew

import (
	"log"
	"time"
)

// Config configures the package manager
type Config struct {
	APIURL      string        // Default: https://formulae.brew.sh/api
	RegistryURL string        // Default: https://ghcr.io/v2/homebrew/core
	InstallPath string        // Default: /usr/local (Intel) or /opt/homebrew (ARM)
	Timeout     time.Duration
	Debug       bool        // Enable debug logging
	Logger      *log.Logger // Custom logger (optional)
}

// PackageManager handles Homebrew package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
}

// FormulaInfo contains metadata about a Homebrew formula from the JSON API
type FormulaInfo struct {
	Name        string                 `json:"name"`
	FullName    string                 `json:"full_name"`
	Description string                 `json:"desc"`
	Homepage    string                 `json:"homepage"`
	License     string                 `json:"license"`
	Versions    FormulaVersions        `json:"versions"`
	Bottle      BottleInfo             `json:"bottle"`
	URLs        map[string]interface{} `json:"urls"`
}

// FormulaVersions contains version information
type FormulaVersions struct {
	Stable string `json:"stable"`
	Head   string `json:"head"`
	Bottle bool   `json:"bottle"`
}

// BottleInfo contains bottle metadata
type BottleInfo struct {
	Stable BottleStable `json:"stable"`
}

// BottleStable contains stable bottle information
type BottleStable struct {
	Rebuild int                        `json:"rebuild"`
	RootURL string                     `json:"root_url"`
	Files   map[string]BottleFileInfo  `json:"files"`
}

// BottleFileInfo contains information about a specific bottle file
type BottleFileInfo struct {
	Cellar string `json:"cellar"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// OCIManifest represents the OCI manifest structure
type OCIManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Manifests     []OCIPlatform     `json:"manifests"`
}

// OCIPlatform represents a platform-specific manifest
type OCIPlatform struct {
	MediaType   string                 `json:"mediaType"`
	Digest      string                 `json:"digest"`
	Size        int                    `json:"size"`
	Platform    PlatformInfo           `json:"platform"`
	Annotations map[string]string      `json:"annotations"`
}

// PlatformInfo contains platform details
type PlatformInfo struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	OSVersion    string `json:"os.version"`
}

// Package represents a Homebrew package
type Package struct {
	Name     string
	Version  string
	Formula  string   // Formula name
	Platform Platform // Target platform
}

// DownloadOptions configures package download and extraction
type DownloadOptions struct {
	Formula     string   // Required: formula name (e.g., "wget")
	Version     string   // Optional: specific version (uses latest stable if empty)
	Platform    Platform // Target platform (auto-detected if empty)
	Extract     bool     // Whether to extract the tarball (default: true)
	KeepArchive bool     // Whether to keep the .tar.gz after extraction (default: false)
	VerifyHash  bool     // Whether to verify SHA256 hash (default: true)
}