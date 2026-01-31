// pkg/registry/registry.go
package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Entry represents a single deps/<name>/index.toml file
type Entry struct {
	Name     string            `toml:"name"`
	Libs     []string          `toml:"libs"`
	Backends map[string]string `toml:"backends"`
}

// Registry provides lookup into the cached deps/ folder
type Registry struct {
	depsDir string
}

// New creates a Registry pointed at the cached deps directory
func New(cacheDir string) *Registry {
	return &Registry{
		depsDir: filepath.Join(cacheDir, "deps"),
	}
}

// Resolve takes a canonical package name and a backend,
// returns the backend-specific package name.
// e.g. Resolve("sqlite3", "apt") -> "libsqlite3-dev"
func (r *Registry) Resolve(name string, backend string) (string, error) {
	entry, err := r.Load(name)
	if err != nil {
		return "", err
	}

	pkgName, ok := entry.Backends[backend]
	if !ok {
		return "", fmt.Errorf("registry: package '%s' has no entry for backend '%s'", name, backend)
	}

	return pkgName, nil
}

// Load reads and parses deps/<name>/index.toml.
// This is the primary method for retrieving package metadata.
func (r *Registry) Load(name string) (*Entry, error) {
	if _, err := os.Stat(r.depsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("registry: deps not found, run sync first")
	}

	path := filepath.Join(r.depsDir, name, "index.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		// Check if the directory exists, to give a better error message.
		dirPath := filepath.Dir(path)
		if _, statErr := os.Stat(dirPath); statErr == nil {
			return nil, fmt.Errorf("registry: found package '%s' directory, but missing index.toml", name)
		}
		return nil, fmt.Errorf("registry: package '%s' not found", name)
	}

	var entry Entry
	if _, err := toml.Decode(string(data), &entry); err != nil {
		return nil, fmt.Errorf("registry: failed to parse '%s': %w", name, err)
	}

	return &entry, nil
}