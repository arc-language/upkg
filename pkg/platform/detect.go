// pkg/platform/detect.go
package platform

import (
	"fmt"
	"runtime"
)

// Platform represents the detected system platform
type Platform struct {
	OS           string   // linux, darwin, windows
	Arch         string   // amd64, arm64, 386, arm
	Available    []string // Available package managers
	Preferred    string   // Preferred package manager
}

// Detect detects the current platform and available package managers
func Detect() (*Platform, error) {
	p := &Platform{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Available: []string{},
	}

	// Check which package managers are available
	if commandExists("nix-env") || commandExists("nix") {
		p.Available = append(p.Available, "nix")
	}
	
	if commandExists("brew") {
		p.Available = append(p.Available, "brew")
	}

	// Determine preferred package manager based on OS
	switch p.OS {
	case "darwin":
		if contains(p.Available, "brew") {
			p.Preferred = "brew"
		} else if contains(p.Available, "nix") {
			p.Preferred = "nix"
		}
	case "linux":
		if contains(p.Available, "nix") {
			p.Preferred = "nix"
		} else if contains(p.Available, "brew") {
			p.Preferred = "brew"
		}
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", p.OS)
	}

	if p.Preferred == "" && len(p.Available) > 0 {
		p.Preferred = p.Available[0]
	}

	return p, nil
}

// String returns a string representation of the platform
func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s (available: %v, preferred: %s)", 
		p.OS, p.Arch, p.Available, p.Preferred)
}