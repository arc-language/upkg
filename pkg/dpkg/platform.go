// pkg/dpkg/platform.go
package dpkg

import (
	"fmt"
	"runtime"
)

// Architecture represents a Debian architecture
type Architecture string

const (
	// Common architectures
	ArchAmd64   Architecture = "amd64"   // x86_64
	ArchI386    Architecture = "i386"    // x86 32-bit
	ArchArm64   Architecture = "arm64"   // ARM 64-bit
	ArchArmhf   Architecture = "armhf"   // ARM hard float
	ArchArmel   Architecture = "armel"   // ARM soft float
	ArchPpc64el Architecture = "ppc64el" // PowerPC 64-bit little endian
	ArchS390x   Architecture = "s390x"   // IBM S/390
	ArchMips64el Architecture = "mips64el"
	ArchRiscv64 Architecture = "riscv64"
	ArchAll     Architecture = "all" // Architecture-independent
)

// AllArchitectures contains all supported Debian architectures
var AllArchitectures = []Architecture{
	ArchAmd64,
	ArchI386,
	ArchArm64,
	ArchArmhf,
	ArchArmel,
	ArchPpc64el,
	ArchS390x,
	ArchMips64el,
	ArchRiscv64,
	ArchAll,
}

// DetectArchitecture automatically detects the current architecture
func DetectArchitecture() (Architecture, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos != "linux" {
		return "", fmt.Errorf("dpkg backend only supports Linux, got: %s", goos)
	}

	switch goarch {
	case "amd64":
		return ArchAmd64, nil
	case "386":
		return ArchI386, nil
	case "arm64":
		return ArchArm64, nil
	case "arm":
		// Default to armhf for ARM 32-bit
		return ArchArmhf, nil
	case "ppc64le":
		return ArchPpc64el, nil
	case "s390x":
		return ArchS390x, nil
	case "mips64le":
		return ArchMips64el, nil
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

// IsMultiArch checks if this is a multi-arch capable architecture
func (a Architecture) IsMultiArch() bool {
	return a == ArchAmd64 || a == ArchI386 || a == ArchArm64 || a == ArchArmhf
}