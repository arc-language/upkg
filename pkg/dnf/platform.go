// pkg/dnf/platform.go
package dnf

import (
	"fmt"
	"runtime"
)

// Architecture represents a Fedora/RPM architecture
type Architecture string

const (
	// Common architectures
	ArchX86_64  Architecture = "x86_64"  // x86 64-bit
	ArchI686    Architecture = "i686"    // x86 32-bit
	ArchAarch64 Architecture = "aarch64" // ARM 64-bit
	ArchArmv7hl Architecture = "armv7hl" // ARM 32-bit hard float
	ArchPpc64le Architecture = "ppc64le" // PowerPC 64-bit little endian
	ArchS390x   Architecture = "s390x"   // IBM S/390
	ArchNoarch  Architecture = "noarch"  // Architecture-independent
)

// AllArchitectures contains all supported Fedora architectures
var AllArchitectures = []Architecture{
	ArchX86_64,
	ArchI686,
	ArchAarch64,
	ArchArmv7hl,
	ArchPpc64le,
	ArchS390x,
	ArchNoarch,
}

// DetectArchitecture automatically detects the current architecture
func DetectArchitecture() (Architecture, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos != "linux" {
		return "", fmt.Errorf("dnf backend only supports Linux, got: %s", goos)
	}

	switch goarch {
	case "amd64":
		return ArchX86_64, nil
	case "386":
		return ArchI686, nil
	case "arm64":
		return ArchAarch64, nil
	case "arm":
		return ArchArmv7hl, nil
	case "ppc64le":
		return ArchPpc64le, nil
	case "s390x":
		return ArchS390x, nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}
}

// String returns the string representation of the architecture
func (a Architecture) String() string {
	return string(a)
}

// IsValid checks if the architecture is valid
func (a Architecture) IsValid() bool {
	for _, valid := range AllArchitectures {
		if a == valid {
			return true
		}
	}
	return false
}