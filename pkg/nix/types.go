// pkg/nix/types.go
package nix

import (
	"log"
	"time"
)

// Package represents an entry in the JSON index
type Package struct {
	Attribute   string `json:"Attribute"`
	NameVersion string `json:"NameVersion"`
	StorePath   string `json:"StorePath"`
}

// Config configures the package manager
type Config struct {
	CacheURL    string        // Default: https://cache.nixos.org
	InstallPath string        // Default: /nix/store
	CachePath   string        // Location of local cache/index files
	Timeout     time.Duration
	Debug       bool          // Enable debug logging
	Logger      *log.Logger   // Custom logger (optional)
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