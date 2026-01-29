// cmd/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/arc-language/upkg"
)

func main() {
	var (
		backendName = flag.String("backend", "auto", "Backend to use (auto, nix, brew, dpkg)")
		pkgName     = flag.String("package", "", "Package name to download")
		pkgVersion  = flag.String("version", "", "Package version (optional)")
		platform    = flag.String("platform", "", "Target platform/architecture (optional)")
		info        = flag.Bool("info", false, "Show package info instead of downloading")
		search      = flag.String("search", "", "Search for packages by keyword")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		installPath = flag.String("install-path", "", "Custom install path")
		noExtract   = flag.Bool("no-extract", false, "Download only, don't extract")
		keepArchive = flag.Bool("keep-archive", false, "Keep archive file after extraction")
		noVerify    = flag.Bool("no-verify", false, "Skip hash verification")
	)
	flag.Parse()

	// Check for required arguments
	if *pkgName == "" && *search == "" {
		fmt.Println("Usage: upkg -package=<name> [-version=<version>] [-backend=<auto|nix|brew|dpkg>] [-info]")
		fmt.Println("   or: upkg -search=<keyword> [-backend=<auto|nix|brew|dpkg>]")
		fmt.Println()
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create configuration
	config := upkg.DefaultConfig()
	config.Debug = *debug
	if *debug {
		config.Logger = log.New(os.Stdout, "[upkg] ", log.LstdFlags)
	}
	if *installPath != "" {
		config.InstallPath = *installPath
	}

	// Determine backend type
	var backendType upkg.BackendType
	switch *backendName {
	case "auto":
		backendType = upkg.BackendAuto
	case "nix":
		backendType = upkg.BackendNix
	case "brew":
		backendType = upkg.BackendBrew
	case "dpkg":
		backendType = upkg.BackendDpkg
	default:
		fmt.Printf("Unknown backend: %s\n", *backendName)
		fmt.Println("Available backends: auto, nix, brew, dpkg")
		os.Exit(1)
	}

	// Create manager
	mgr, err := upkg.NewManager(backendType, config)
	if err != nil {
		fmt.Printf("Error initializing manager: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	fmt.Printf("Using backend: %s\n", mgr.Backend())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Handle search
	if *search != "" {
		fmt.Printf("Searching for packages matching: %s\n\n", *search)
		results, err := mgr.Search(ctx, *search)
		if err != nil {
			fmt.Printf("Error searching packages: %v\n", err)
			os.Exit(1)
		}

		if len(results) == 0 {
			fmt.Println("No packages found")
			os.Exit(0)
		}

		fmt.Printf("Found %d package(s):\n\n", len(results))
		for i, pkg := range results {
			if i >= 20 {
				fmt.Printf("... and %d more results (showing first 20)\n", len(results)-20)
				break
			}
			fmt.Printf("%s (%s)\n", pkg.Name, pkg.Version)
			if pkg.Description != "" {
				// Limit description to first line
				desc := pkg.Description
				if idx := findNewline(desc); idx > 0 {
					desc = desc[:idx]
				}
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
				fmt.Printf("  %s\n", desc)
			}
			if len(pkg.Platforms) > 0 {
				fmt.Printf("  Platforms: %v\n", pkg.Platforms)
			}
			fmt.Println()
		}
		return
	}

	// Show info or download
	if *info {
		// Get package info
		pkgInfo, err := mgr.GetInfo(ctx, *pkgName)
		if err != nil {
			fmt.Printf("Error getting package info: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nPackage Information:\n")
		fmt.Printf("═══════════════════════════════════════════════════════\n")
		fmt.Printf("  Name:        %s\n", pkgInfo.Name)
		fmt.Printf("  Version:     %s\n", pkgInfo.Version)
		fmt.Printf("  Backend:     %s\n", pkgInfo.Backend)
		if pkgInfo.Description != "" {
			fmt.Printf("  Description: %s\n", pkgInfo.Description)
		}
		if pkgInfo.Homepage != "" {
			fmt.Printf("  Homepage:    %s\n", pkgInfo.Homepage)
		}
		if pkgInfo.License != "" {
			fmt.Printf("  License:     %s\n", pkgInfo.License)
		}
		if len(pkgInfo.Platforms) > 0 {
			fmt.Printf("  Platforms:   %v\n", pkgInfo.Platforms)
		}
		if len(pkgInfo.Outputs) > 0 {
			fmt.Printf("  Outputs:     %v\n", pkgInfo.Outputs)
		}
		fmt.Printf("═══════════════════════════════════════════════════════\n")
	} else {
		// Download package
		pkg := &upkg.Package{
			Name:    *pkgName,
			Version: *pkgVersion,
		}

		opts := &upkg.DownloadOptions{
			Platform: *platform,
		}

		// Set boolean options
		if *noExtract {
			extract := false
			opts.Extract = &extract
		}
		if *keepArchive {
			keep := true
			opts.KeepArchive = &keep
		}
		if *noVerify {
			verify := false
			opts.VerifyHash = &verify
		}

		fmt.Printf("\nDownloading package: %s\n", *pkgName)
		if *pkgVersion != "" {
			fmt.Printf("  Version:  %s\n", *pkgVersion)
		}
		if *platform != "" {
			fmt.Printf("  Platform: %s\n", *platform)
		}
		fmt.Printf("  Install:  %s\n", config.InstallPath)
		fmt.Println()

		err := mgr.Download(ctx, pkg, opts)
		if err != nil {
			fmt.Printf("\n✗ Error downloading package: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✓ Successfully downloaded and installed %s\n", *pkgName)
		
		// Show some helpful next steps based on backend
		switch mgr.Backend() {
		case "brew", "dpkg":
			fmt.Printf("\nInstallation location: %s\n", config.InstallPath)
			fmt.Printf("You may need to add the following to your PATH:\n")
			fmt.Printf("  export PATH=\"%s/bin:$PATH\"\n", config.InstallPath)
		case "nix":
			fmt.Printf("\nNix package installed. Check your Nix profile for binaries.\n")
		}
	}
}

// findNewline finds the index of the first newline character
func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}