// cmd/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/arc-language/upkg"
)

func main() {
	var (
		backendName = flag.String("backend", "auto", "Backend to use (auto, nix, brew, dpkg, apt, apk, dnf, choco, pacman)")
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
		fmt.Println("upkg - Universal Package Manager")
		fmt.Println()
		fmt.Println("Usage: upkg -package=<name> [-version=<version>] [-backend=<type>] [-info]")
		fmt.Println("   or: upkg -search=<keyword> [-backend=<type>]")
		fmt.Println()
		fmt.Println("Backends:")
		fmt.Println("  auto   - Automatically detect best backend (default)")
		fmt.Println("  nix    - Nix package manager (Cross-platform)")
		fmt.Println("  brew   - Homebrew package manager (macOS/Linux)")
		fmt.Println("  dpkg   - Debian package manager (Debian-focused)")
		fmt.Println("  apt    - Ubuntu package manager (Ubuntu-focused)")
		fmt.Println("  apk    - Alpine package manager (Alpine Linux)")
		fmt.Println("  dnf    - Fedora package manager (RedHat/Fedora)")
		fmt.Println("  pacman - Arch Linux package manager (Arch/Manjaro)")
		fmt.Println("  choco  - Chocolatey package manager (Windows)")
		fmt.Println()
		fmt.Println("Options:")
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
	case "apt":
		backendType = upkg.BackendApt
	case "apk":
		backendType = upkg.BackendApk
	case "dnf":
		backendType = upkg.BackendDnf
	case "choco":
		backendType = upkg.BackendChoco
	case "pacman":
		backendType = upkg.BackendPacman
	default:
		fmt.Printf("Unknown backend: %s\n", *backendName)
		fmt.Println("Available backends: auto, nix, brew, dpkg, apt, apk, dnf, choco, pacman")
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
			fmt.Printf("📦 %s (%s)\n", pkg.Name, pkg.Version)
			if pkg.Description != "" {
				// Limit description to first line
				desc := pkg.Description
				if idx := findNewline(desc); idx > 0 {
					desc = desc[:idx]
				}
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
				fmt.Printf("   %s\n", desc)
			}
			if len(pkg.Platforms) > 0 {
				fmt.Printf("   Platforms: %v\n", pkg.Platforms)
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

		fmt.Printf("\n")
		fmt.Printf("╔═══════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║                    Package Information                        ║\n")
		fmt.Printf("╠═══════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║ Name:        %-48s ║\n", truncate(pkgInfo.Name, 48))
		fmt.Printf("║ Version:     %-48s ║\n", truncate(pkgInfo.Version, 48))
		fmt.Printf("║ Backend:     %-48s ║\n", truncate(pkgInfo.Backend, 48))
		if pkgInfo.Description != "" {
			descLines := wrapText(pkgInfo.Description, 48)
			fmt.Printf("║ Description: %-48s ║\n", descLines[0])
			for i := 1; i < len(descLines) && i < 3; i++ {
				fmt.Printf("║              %-48s ║\n", descLines[i])
			}
			if len(descLines) > 3 {
				fmt.Printf("║              %-48s ║\n", "...")
			}
		}
		if pkgInfo.Homepage != "" {
			fmt.Printf("║ Homepage:    %-48s ║\n", truncate(pkgInfo.Homepage, 48))
		}
		if pkgInfo.License != "" {
			fmt.Printf("║ License:     %-48s ║\n", truncate(pkgInfo.License, 48))
		}
		if len(pkgInfo.Platforms) > 0 {
			platformStr := strings.Join(pkgInfo.Platforms, ", ")
			fmt.Printf("║ Platforms:   %-48s ║\n", truncate(platformStr, 48))
		}
		if len(pkgInfo.Outputs) > 0 {
			var outputs []string
			for k := range pkgInfo.Outputs {
				outputs = append(outputs, k)
			}
			outputStr := strings.Join(outputs, ", ")
			fmt.Printf("║ Outputs:     %-48s ║\n", truncate(outputStr, 48))
		}
		fmt.Printf("╚═══════════════════════════════════════════════════════════════╝\n")
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

		fmt.Printf("\n")
		fmt.Printf("╔═══════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║                     Downloading Package                       ║\n")
		fmt.Printf("╠═══════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║ Package:  %-51s ║\n", truncate(*pkgName, 51))
		if *pkgVersion != "" {
			fmt.Printf("║ Version:  %-51s ║\n", truncate(*pkgVersion, 51))
		}
		if *platform != "" {
			fmt.Printf("║ Platform: %-51s ║\n", truncate(*platform, 51))
		}
		fmt.Printf("║ Install:  %-51s ║\n", truncate(config.InstallPath, 51))
		fmt.Printf("╚═══════════════════════════════════════════════════════════════╝\n")
		fmt.Println()

		err := mgr.Download(ctx, pkg, opts)
		if err != nil {
			fmt.Printf("\n❌ Error downloading package: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ Successfully downloaded and installed %s\n", *pkgName)

		// Show some helpful next steps based on backend
		switch mgr.Backend() {
		case "brew", "dpkg", "apt", "dnf", "apk", "pacman":
			fmt.Printf("\n📍 Installation location: %s\n", config.InstallPath)
			fmt.Printf("\n💡 You may need to add the following to your PATH:\n")
			fmt.Printf("   export PATH=\"%s/bin:$PATH\"\n", config.InstallPath)
			fmt.Printf("   export PATH=\"%s/usr/bin:$PATH\"\n", config.InstallPath)
			fmt.Printf("\n   Or source this in your shell profile (~/.bashrc, ~/.zshrc):\n")
			fmt.Printf("   echo 'export PATH=\"%s/bin:$PATH\"' >> ~/.bashrc\n", config.InstallPath)
		case "choco":
			fmt.Printf("\n📍 Installation location: %s\n", config.InstallPath)
			fmt.Printf("\n💡 Windows binaries are usually in the tools directory or extracted root.\n")
		case "nix":
			fmt.Printf("\n📍 Nix package installed. Check your Nix profile for binaries.\n")
		}

		// Show what was installed
		fmt.Printf("\n📦 Installed files can be found in:\n")
		switch mgr.Backend() {
		case "dpkg", "apt", "dnf", "pacman":
			fmt.Printf("   %s/usr/bin/     - Executables\n", config.InstallPath)
			fmt.Printf("   %s/usr/lib/     - Libraries\n", config.InstallPath)
			fmt.Printf("   %s/usr/share/   - Shared data\n", config.InstallPath)
		case "apk":
			fmt.Printf("   %s/bin/         - Executables\n", config.InstallPath)
			fmt.Printf("   %s/lib/         - Libraries\n", config.InstallPath)
		case "brew":
			fmt.Printf("   %s/Cellar/      - Bottle files\n", config.InstallPath)
			fmt.Printf("   %s/bin/         - Executables (if linked)\n", config.InstallPath)
		case "choco":
			fmt.Printf("   %s/%s/          - Package content\n", config.InstallPath, *pkgName)
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

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// wrapText wraps text to fit within a specified width
func wrapText(text string, width int) []string {
	var lines []string
	var currentLine string

	words := strings.Fields(text)
	for _, word := range words {
		if len(currentLine)+len(word)+1 <= width {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// Pad lines to width
	for i := range lines {
		if len(lines[i]) < width {
			lines[i] += strings.Repeat(" ", width-len(lines[i]))
		}
	}

	return lines
}