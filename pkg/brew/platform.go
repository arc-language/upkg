// platform.go
package brew

import (
	"fmt"
	"runtime"
)

// Platform represents a Homebrew platform
type Platform string

const (
	// macOS platforms
	PlatformArm64Tahoe    Platform = "arm64_tahoe"    // macOS 26 ARM64
	PlatformTahoe         Platform = "tahoe"          // macOS 26 Intel
	PlatformArm64Sequoia  Platform = "arm64_sequoia"  // macOS 15 ARM64
	PlatformSequoia       Platform = "sequoia"        // macOS 15 Intel
	PlatformArm64Sonoma   Platform = "arm64_sonoma"   // macOS 14 ARM64
	PlatformSonoma        Platform = "sonoma"         // macOS 14 Intel
	PlatformArm64Ventura  Platform = "arm64_ventura"  // macOS 13 ARM64
	PlatformVentura       Platform = "ventura"        // macOS 13 Intel
	PlatformMonterey      Platform = "monterey"       // macOS 12 Intel
	PlatformArm64Monterey Platform = "arm64_monterey" // macOS 12 ARM64
	PlatformBigSur        Platform = "big_sur"        // macOS 11 Intel
	PlatformArm64BigSur   Platform = "arm64_big_sur"  // macOS 11 ARM64

	// Linux platforms
	PlatformX8664Linux   Platform = "x86_64_linux"
	PlatformAarch64Linux Platform = "aarch64_linux"
)

// AllPlatforms contains commonly used Homebrew platforms
var AllPlatforms = []Platform{
	PlatformArm64Tahoe,
	PlatformTahoe,
	PlatformArm64Sequoia,
	PlatformSequoia,
	PlatformArm64Sonoma,
	PlatformSonoma,
	PlatformArm64Ventura,
	PlatformVentura,
	PlatformMonterey,
	PlatformArm64Monterey,
	PlatformBigSur,
	PlatformArm64BigSur,
	PlatformX8664Linux,
	PlatformAarch64Linux,
}

// DetectPlatform automatically detects the current platform
func DetectPlatform() (Platform, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "darwin":
		// Try to detect macOS version
		// For now, default to latest supported versions
		switch goarch {
		case "arm64":
			return PlatformArm64Sequoia, nil
		case "amd64":
			return PlatformSequoia, nil
		default:
			return "", fmt.Errorf("unsupported Darwin architecture: %s", goarch)
		}

	case "linux":
		switch goarch {
		case "amd64":
			return PlatformX8664Linux, nil
		case "arm64":
			return PlatformAarch64Linux, nil
		default:
			return "", fmt.Errorf("unsupported Linux architecture: %s", goarch)
		}

	default:
		return "", fmt.Errorf("unsupported operating system: %s", goos)
	}
}

// String returns the string representation of the platform
func (p Platform) String() string {
	return string(p)
}

// IsValid checks if the platform is a known valid platform
func (p Platform) IsValid() bool {
	for _, valid := range AllPlatforms {
		if p == valid {
			return true
		}
	}
	return false
}

// ToOCI converts Homebrew platform to OCI format (amd64/arm64)
func (p Platform) ToOCI() string {
	s := string(p)
	if len(s) >= 5 && s[:5] == "arm64" {
		return "arm64"
	}
	return "amd64"
}