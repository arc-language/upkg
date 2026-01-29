package zypper

import (
	"fmt"
	"runtime"
)

// DetectArchitecture checks the system architecture
func DetectArchitecture() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos != "linux" {
		// Default fallback for non-linux systems
	}

	switch goarch {
	case "amd64":
		return "x86_64", nil
	case "386":
		return "i586", nil // SUSE often uses i586 for 32-bit
	case "arm64":
		return "aarch64", nil
	case "s390x":
		return "s390x", nil
	case "ppc64le":
		return "ppc64le", nil
	default:
		return "", fmt.Errorf("unsupported architecture for zypper: %s", goarch)
	}
}