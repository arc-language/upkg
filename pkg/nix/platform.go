// platform.go
package nix

import (
	"fmt"
	"runtime"
)

// Platform represents a Nix platform triple
type Platform string

const (
	// Linux platforms
	PlatformX8664Linux   Platform = "x86_64-linux"
	PlatformI686Linux    Platform = "i686-linux"
	PlatformAarch64Linux Platform = "aarch64-linux"
	PlatformArmv7lLinux  Platform = "armv7l-linux"
	PlatformArmv6lLinux  Platform = "armv6l-linux"

	// macOS platforms
	PlatformX8664Darwin   Platform = "x86_64-darwin"
	PlatformAarch64Darwin Platform = "aarch64-darwin"
)

// AllPlatforms contains commonly used Nix platforms
var AllPlatforms = []Platform{
	PlatformX8664Linux,
	PlatformI686Linux,
	PlatformAarch64Linux,
	PlatformArmv7lLinux,
	PlatformArmv6lLinux,
	PlatformX8664Darwin,
	PlatformAarch64Darwin,
}

// DetectPlatform automatically detects the current platform
func DetectPlatform() (Platform, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return PlatformX8664Linux, nil
		case "386":
			return PlatformI686Linux, nil
		case "arm64":
			return PlatformAarch64Linux, nil
		case "arm":
			return PlatformArmv7lLinux, nil
		default:
			return "", fmt.Errorf("unsupported Linux architecture: %s", goarch)
		}

	case "darwin":
		switch goarch {
		case "amd64":
			return PlatformX8664Darwin, nil
		case "arm64":
			return PlatformAarch64Darwin, nil
		default:
			return "", fmt.Errorf("unsupported Darwin architecture: %s", goarch)
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