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
		backendName = flag.String("backend", "auto", "Backend to use (auto, nix, brew)")
		pkgName     = flag.String("package", "", "Package name to download")
		pkgVersion  = flag.String("version", "", "Package version (optional)")
		info        = flag.Bool("info", false, "Show package info instead of downloading")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		installPath = flag.String("install-path", "", "Custom install path")
	)
	flag.Parse()

	if *pkgName == "" {
		fmt.Println("Usage: upkg -package=<name> [-version=<version>] [-backend=<auto|nix|brew>] [-info]")
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
	default:
		fmt.Printf("Unknown backend: %s\n", *backendName)
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

	// Show info or download
	if *info {
		// Get package info
		pkgInfo, err := mgr.GetInfo(ctx, *pkgName)
		if err != nil {
			fmt.Printf("Error getting package info: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nPackage Information:\n")
		fmt.Printf("  Name:        %s\n", pkgInfo.Name)
		fmt.Printf("  Version:     %s\n", pkgInfo.Version)
		fmt.Printf("  Description: %s\n", pkgInfo.Description)
		fmt.Printf("  Homepage:    %s\n", pkgInfo.Homepage)
		fmt.Printf("  License:     %s\n", pkgInfo.License)
		fmt.Printf("  Platforms:   %v\n", pkgInfo.Platforms)
		if len(pkgInfo.Outputs) > 0 {
			fmt.Printf("  Outputs:     %v\n", pkgInfo.Outputs)
		}
	} else {
		// Download package
		pkg := &upkg.Package{
			Name:    *pkgName,
			Version: *pkgVersion,
		}

		fmt.Printf("Downloading package: %s\n", *pkgName)
		if *pkgVersion != "" {
			fmt.Printf("  Version: %s\n", *pkgVersion)
		}

		err := mgr.Download(ctx, pkg, &upkg.DownloadOptions{})
		if err != nil {
			fmt.Printf("Error downloading package: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Successfully downloaded and installed %s\n", *pkgName)
	}
}