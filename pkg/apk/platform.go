// pkg/apk/platform.go
package apk

import (
	"fmt"
	"runtime"
)

// Architecture represents an Alpine architecture
type Architecture string

const (
	// Common architectures
	ArchX86_64   Architecture = "x86_64"  // x86 64-bit (Intel/AMD)
	ArchX86      Architecture = "x86"     // x86 32-bit
	ArchAarch64  Architecture = "aarch64" // ARM 64-bit
	ArchArmhf    Architecture = "armhf"   // ARM hard float
	ArchArmv7    Architecture = "armv7"   // ARMv7
	ArchPpc64le  Architecture = "ppc64le" // PowerPC 64-bit little endian
	ArchS390x    Architecture = "s390x"   // IBM S/390
	ArchRiscv64  Architecture = "riscv64" // RISC-V 64-bit
	ArchNoarch   Architecture = "noarch"  // Architecture-independent
)

// AllArchitectures contains all supported Alpine architectures
var AllArchitectures = []Architecture{
	ArchX86_64,
	ArchX86,
	ArchAarch64,
	ArchArmhf,
	ArchArmv7,
	ArchPpc64le,
	ArchS390x,
	ArchRiscv64,
	ArchNoarch,
}

// DetectArchitecture automatically detects the current architecture
func DetectArchitecture() (Architecture, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos != "linux" {
		return "", fmt.Errorf("apk backend only supports Linux, got: %s", goos)
	}

	switch goarch {
	case "amd64":
		return ArchX86_64, nil
	case "386":
		return ArchX86, nil
	case "arm64":
		return ArchAarch64, nil
	case "arm":
		// Default to armv7 for ARM 32-bit
		return ArchArmv7, nil
	case "ppc64le":
		return ArchPpc64le, nil
	case "s390x":
		return ArchS390x, nil
	case "riscv64":
		return ArchRiscv64, nil
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