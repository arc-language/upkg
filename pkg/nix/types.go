// types.go
package nix

import (
	"log"
	"time"
)

// Config configures the package manager
type Config struct {
	CacheURL    string        // Default: https://cache.nixos.org
	InstallPath string        // Default: /nix/store
	Timeout     time.Duration
	Debug       bool          // Enable debug logging
	Logger      *log.Logger   // Custom logger (optional)
}

// PackageManager handles Nix package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
}

// NARInfo contains metadata about a Nix package
type NARInfo struct {
	StorePath   string
	URL         string
	Compression string
	FileHash    string
	FileSize    int64
	NarHash     string
	NarSize     int64
	References  []string
	Deriver     string
	Signature   string
}

// DownloadOptions configures package download and extraction
type DownloadOptions struct {
	Outputs      []string // Which outputs to download (e.g., ["bin", "dev"]). Empty = all outputs
	Compression  string   // xz, bzip2, none
	Extract      bool     // Whether to extract the NAR archive (default: true)
	KeepArchive  bool     // Whether to keep the .nar.xz file after extraction (default: false)
	VerifyHash   bool     // Whether to verify file hash after download (default: true)
}