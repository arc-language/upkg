package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Entry represents a single deps/<name>/index.toml file
type Entry struct {
	Aliases  []string          `toml:"aliases"`
	Backends map[string]string `toml:"backends"`
}

// Registry provides lookup into the cached deps/ folder
type Registry struct {
	depsDir string
	aliases map[string]string // alias -> canonical name, built lazily
}

// New creates a Registry pointed at the cached deps directory
func New(cacheDir string) *Registry {
	return &Registry{
		depsDir: filepath.Join(cacheDir, "deps"),
	}
}

// Resolve takes a name (canonical or alias) and a backend,
// returns the backend-specific package name.
// e.g. Resolve("sqlite", "apt") -> "libsqlite3-dev"
func (r *Registry) Resolve(name string, backend string) (string, error) {
	canonical, err := r.resolveCanonical(name)
	if err != nil {
		return "", err
	}

	entry, err := r.Load(canonical)
	if err != nil {
		return "", err
	}

	pkgName, ok := entry.Backends[backend]
	if !ok {
		return "", fmt.Errorf("registry: package '%s' has no entry for backend '%s'", canonical, backend)
	}

	return pkgName, nil
}

// Load reads and parses deps/<name>/index.toml
func (r *Registry) Load(name string) (*Entry, error) {
	path := filepath.Join(r.depsDir, name, "index.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("registry: package '%s' not found", name)
	}

	var entry Entry
	if _, err := toml.Decode(string(data), &entry); err != nil {
		return nil, fmt.Errorf("registry: failed to parse '%s': %w", name, err)
	}

	return &entry, nil
}

// resolveCanonical checks if name is a folder in deps/,
// if not it scans aliases to find the canonical name
func (r *Registry) resolveCanonical(name string) (string, error) {
	if _, err := os.Stat(r.depsDir); os.IsNotExist(err) {
		return "", fmt.Errorf("registry: deps not found, run sync first")
	}

	// Direct match — folder exists
	candidate := filepath.Join(r.depsDir, name)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return name, nil
	}

	// Not a direct match, build alias map if needed and look up
	if r.aliases == nil {
		if err := r.buildAliasMap(); err != nil {
			return "", err
		}
	}

	canonical, ok := r.aliases[name]
	if !ok {
		return "", fmt.Errorf("registry: package '%s' not found", name)
	}

	return canonical, nil
}

// buildAliasMap scans all deps/ entries and builds alias -> canonical name map
func (r *Registry) buildAliasMap() error {
	r.aliases = make(map[string]string)

	entries, err := os.ReadDir(r.depsDir)
	if err != nil {
		return fmt.Errorf("registry: failed to read deps directory: %w", err)
	}

	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}

		canonical := dir.Name()
		entry, err := r.Load(canonical)
		if err != nil {
			continue // skip broken entries
		}

		for _, alias := range entry.Aliases {
			r.aliases[alias] = canonical
		}
	}

	return nil
}