// constants.go
package nix

const (
	// DefaultCacheURL is the official Nix binary cache
	DefaultCacheURL = "https://cache.nixos.org"

	// DefaultSearchURL is the package search endpoint
	DefaultSearchURL = "https://search.nixos.org"

	// DefaultInstallPath is where Nix typically stores packages
	DefaultInstallPath = "/nix/store"

	// DefaultLocalCache is where Nix caches downloaded files
	DefaultLocalCache = "~/.cache/nix"

	// CompressionXZ uses xz compression
	CompressionXZ = "xz"

	// CompressionBZip2 uses bzip2 compression
	CompressionBZip2 = "bzip2"

	// CompressionNone uses no compression
	CompressionNone = "none"
)

var (
	// DefaultPlatforms contains supported platforms
	DefaultPlatforms = []string{
		"x86_64-linux",
		"aarch64-linux",
		"x86_64-darwin",
		"aarch64-darwin",
	}
)