// pkg/registry/registry.go
package registry

import (
	"fmt"
	"sync"

	"github.com/arc-language/upkg/pkg/core"
	brewbackend "github.com/arc-language/upkg/pkg/backends/brew"
	nixbackend "github.com/arc-language/upkg/pkg/backends/nix"
)

var (
	mu       sync.RWMutex
	backends = make(map[string]func(installPath string, debug bool) core.PackageManager)
)

func init() {
	// Register built-in backends
	Register("nix", func(installPath string, debug bool) core.PackageManager {
		return nixbackend.NewAdapter(installPath, debug)
	})
	
	Register("brew", func(installPath string, debug bool) core.PackageManager {
		return brewbackend.NewAdapter(installPath, debug)
	})
}

// Register registers a new package manager backend
func Register(name string, factory func(installPath string, debug bool) core.PackageManager) {
	mu.Lock()
	defer mu.Unlock()
	backends[name] = factory
}

// Get returns a package manager backend by name
func Get(name string, installPath string, debug bool) (core.PackageManager, error) {
	mu.RLock()
	factory, exists := backends[name]
	mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown backend: %s", name)
	}

	return factory(installPath, debug), nil
}

// Available returns a list of registered backend names
func Available() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	return names
}