// pkg/backend/nix.go
package backend

import (
	"context"
	"fmt"
	"strings"

	"github.com/arc-language/upkg/pkg/nix"
)

// NixBackend implements the Backend interface for Nix
type NixBackend struct {
	manager *nix.PackageManager
	config  *Config
}

// NewNixBackend creates a new Nix backend
func NewNixBackend(config *Config) (*NixBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	nixConfig := &nix.Config{
		CacheURL:    config.Nix.CacheURL,
		InstallPath: config.InstallPath,
		CachePath:   config.CachePath, // Pass the cache path for index loading
		Timeout:     config.Timeout,
		Debug:       config.Debug,
		Logger:      config.Logger,
	}

	if nixConfig.Logger == nil && config.Debug {
		nixConfig.Logger = config.Logger
	}

	manager := nix.NewPackageManager(nixConfig)

	return &NixBackend{
		manager: manager,
		config:  config,
	}, nil
}

// Download downloads a package using Nix
// Note: pkg.Name should be the Nix attribute name (e.g., "ffmpeg", "python313Packages.numpy")
func (b *NixBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	// Build the attribute name
	// If pkg.Name is already a full attribute (contains '.'), use it as-is
	// Otherwise, use it directly as the attribute
	attribute := pkg.Name

	nixOpts := &nix.DownloadOptions{
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	// If specific outputs are requested via pkg.Output, use it
	if pkg.Output != "" {
		nixOpts.Outputs = []string{pkg.Output}
	}

	return b.manager.Download(ctx, attribute, nixOpts)
}

// GetInfo retrieves package information from Nix
func (b *NixBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	// Lookup package in the loaded registry
	pkg, err := b.manager.LookupPackage(name)
	if err != nil {
		return nil, fmt.Errorf("looking up package: %w", err)
	}

	// Parse the store path to get all available outputs
	outputs := parseStorePath(pkg.StorePath)

	return &PackageInfo{
		Name:        pkg.Attribute,
		Version:     pkg.NameVersion,
		Description: "", // Not available in static registry
		Outputs:     outputs,
		Platforms:   []string{"x86_64-linux"}, // Only supported platform currently
		Backend:     "nix",
	}, nil
}

// Search searches for packages in the Nix registry
// This is a simple substring match against attribute names
func (b *NixBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	query = strings.ToLower(query)
	var results []*PackageInfo

	// Get the package registry from the manager instance
	registry := b.manager.GetPackageRegistry()
	
	// Simple linear search through the registry
	for _, pkg := range registry {
		if strings.Contains(strings.ToLower(pkg.Attribute), query) ||
			strings.Contains(strings.ToLower(pkg.NameVersion), query) {
			
			outputs := parseStorePath(pkg.StorePath)
			
			results = append(results, &PackageInfo{
				Name:        pkg.Attribute,
				Version:     pkg.NameVersion,
				Description: "",
				Outputs:     outputs,
				Platforms:   []string{"x86_64-linux"},
				Backend:     "nix",
			})

			// Limit results to avoid overwhelming output
			if len(results) >= 100 {
				break
			}
		}
	}

	return results, nil
}

// Name returns the backend name
func (b *NixBackend) Name() string {
	return "nix"
}

// Close cleans up resources
func (b *NixBackend) Close() error {
	return nil
}

// parseStorePath parses a StorePath field into a map of outputs
// Format: "bin=/nix/store/hash-name;dev=/nix/store/hash-name;/nix/store/hash-name"
func parseStorePath(storePath string) map[string]string {
	outputs := make(map[string]string)
	parts := strings.Split(storePath, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "=") {
			// Named output: "bin=/nix/store/hash-name"
			kv := strings.SplitN(part, "=", 2)
			outputName := kv[0]
			fullPath := kv[1]

			// Extract hash from "/nix/store/hash-name-version"
			pathWithoutPrefix := strings.TrimPrefix(fullPath, "/nix/store/")
			hashParts := strings.SplitN(pathWithoutPrefix, "-", 2)
			if len(hashParts) > 0 {
				outputs[outputName] = hashParts[0]
			}
		} else {
			// Default output: "/nix/store/hash-name"
			pathWithoutPrefix := strings.TrimPrefix(part, "/nix/store/")
			hashParts := strings.SplitN(pathWithoutPrefix, "-", 2)
			if len(hashParts) > 0 {
				outputs["out"] = hashParts[0]
			}
		}
	}

	return outputs
}