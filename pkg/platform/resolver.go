// pkg/platform/resolver.go
package platform

import (
	"fmt"

	"github.com/arc-language/upkg/pkg/core"
	"github.com/arc-language/upkg/pkg/registry"
)

// ResolveBackend resolves which backend to use based on platform and config
func ResolveBackend(platform *Platform, config *core.Config) (core.PackageManager, error) {
	var backendName string

	// Priority:
	// 1. User-specified backend in config
	// 2. Platform preferred backend
	// 3. First available backend
	if config.DefaultBackend != "" {
		backendName = config.DefaultBackend
	} else if platform.Preferred != "" {
		backendName = platform.Preferred
	} else if len(platform.Available) > 0 {
		backendName = platform.Available[0]
	} else {
		return nil, fmt.Errorf("no package managers available")
	}

	// Check if backend is available
	if !contains(platform.Available, backendName) {
		return nil, fmt.Errorf("backend '%s' is not available on this system", backendName)
	}

	// Get backend from registry
	backend, err := registry.Get(backendName)
	if err != nil {
		return nil, fmt.Errorf("getting backend '%s': %w", backendName, err)
	}

	return backend, nil
}