// pkg/apk/index.go
package apk

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// updatePackageIndex downloads and indexes packages from all repositories
func (pm *PackageManager) updatePackageIndex(ctx context.Context, arch Architecture) error {
	// Check if cache is still valid
	if time.Since(pm.cache.lastUpdate) < pm.cache.cacheDuration && len(pm.cache.packages) > 0 {
		pm.logger.Printf("Using cached package index (age: %v)", time.Since(pm.cache.lastUpdate))
		return nil
	}

	pm.logger.Printf("Fetching package index from repositories...")

	// Clear/Init cache
	pm.cache.packages = make(map[string]*PackageInfo)
	pm.cache.providers = make(map[string][]*PackageInfo)

	// Repositories to index (order matters for preference)
	repositories := []string{"main", "community"}
	
	// If config specifies a repo that isn't standard, add it (e.g. testing)
	if pm.config.Repository != "main" && pm.config.Repository != "community" {
		repositories = append(repositories, pm.config.Repository)
	}

	totalPackages := 0
	var lastErr error

	for _, repo := range repositories {
		// Construct URL for APKINDEX.tar.gz
		// e.g. https://dl-cdn.alpinelinux.org/alpine/v3.19/main/x86_64/APKINDEX.tar.gz
		url := fmt.Sprintf("%s/%s/%s/%s/APKINDEX.tar.gz",
			pm.config.RepositoryURL,
			pm.config.Branch,
			repo,
			arch)

		pm.logger.Printf("  Fetching %s repository: %s", repo, url)

		// Download APKINDEX
		resp, err := pm.client.Get(ctx, url)
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to fetch %s repository: %v", repo, err)
			lastErr = err
			continue
		}

		// Parse packages using existing parser
		packages, err := ParseAPKINDEX(resp.Body)
		resp.Body.Close()
		if err != nil {
			pm.logger.Printf("  ⚠️  Warning: failed to parse %s repository: %v", repo, err)
			lastErr = err
			continue
		}

		// Index the packages
		count := 0
		for _, pkg := range packages {
			pkg.Repository = repo // Store origin repo
			
			// 1. Map Name -> Package
			// Note: This overwrites if same package exists in multiple repos. 
			// Since we iterate main -> community, community might overwrite main if duplicates exist.
			// Usually we want main to take precedence.
			if _, exists := pm.cache.packages[pkg.Package]; !exists {
				pm.cache.packages[pkg.Package] = pkg
			}

			// 2. Map Provides -> Package (The Reverse Lookup)
			for _, provided := range pkg.Provides {
				// provided string is like "cmd:sh" or "so:libssl.so.3"
				pm.cache.providers[provided] = append(pm.cache.providers[provided], pkg)
			}
			
			// Also map the package name itself as a provider
			// This handles cases where a dependency is just the package name
			pm.cache.providers[pkg.Package] = append(pm.cache.providers[pkg.Package], pkg)

			count++
		}

		pm.logger.Printf("  ✓ Indexed %d packages from %s", count, repo)
		totalPackages += count
	}

	if totalPackages == 0 {
		if lastErr != nil {
			return fmt.Errorf("failed to fetch any packages: %w", lastErr)
		}
		return fmt.Errorf("no packages found in any repository")
	}

	pm.logger.Printf("  Total packages indexed: %d", totalPackages)
	pm.logger.Printf("  Total unique providers: %d", len(pm.cache.providers))
	pm.cache.lastUpdate = time.Now()

	return nil
}

// pickBestProvider selects the best package from a list of providers
func (pm *PackageManager) pickBestProvider(providers []*PackageInfo) *PackageInfo {
	if len(providers) == 0 {
		return nil
	}
	
	// 1. If only one, return it
	if len(providers) == 1 {
		return providers[0]
	}

	// 2. Preference: Prefer 'main' repository over others
	for _, pkg := range providers {
		if pkg.Repository == "main" {
			return pkg
		}
	}

	// 3. Preference: Prefer 'community' over testing/others
	for _, pkg := range providers {
		if pkg.Repository == "community" {
			return pkg
		}
	}

	// 4. Fallback: Shortest name? (Heuristic: "bash" is better than "bash-doc")
	// Or just return the first one
	return providers[0]
}

// resolveVirtualPackage attempts to find a package that provides the requested virtual name
func (pm *PackageManager) resolveVirtualPackage(name string) (*PackageInfo, error) {
	// Direct lookup in providers map
	if providers, ok := pm.cache.providers[name]; ok {
		best := pm.pickBestProvider(providers)
		if best != nil {
			return best, nil
		}
	}
	
	// Fallback: try stripping version from request (e.g. "so:libssl.so.3=3.0" -> "so:libssl.so.3")
	// The providers map is built with version stripped (mostly), but if request has version:
	if idx := strings.IndexAny(name, "=<>~"); idx != -1 {
		cleanName := name[:idx]
		if providers, ok := pm.cache.providers[cleanName]; ok {
			best := pm.pickBestProvider(providers)
			if best != nil {
				return best, nil
			}
		}
	}

	return nil, fmt.Errorf("no provider found for %s", name)
}