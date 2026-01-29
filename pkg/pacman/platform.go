package pacman

import (
	"fmt"
	"runtime"
)

// DetectArchitecture checks the system architecture
func DetectArchitecture() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goos != "linux" {
		// We allow running on non-Linux for downloading purposes, 
		// but warn about architecture mismatch if strictly needed.
		// For now, return default.
	}

	switch goarch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil // Arch Linux ARM uses aarch64
	default:
		return "", fmt.Errorf("unsupported architecture for pacman: %s", goarch)
	}
}