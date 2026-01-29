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

// Common flags structure to share between subcommands
type commonFlags struct {
	backendName string
	debug       bool
	installPath string
}

func main() {
	// Custom usage message
	flag.Usage = func() {
		printUsage()
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Subcommand switching
	switch os.Args[1] {
	case "install", "i":
		runInstall(os.Args[2:])
	case "search", "s":
		runSearch(os.Args[2:])
	case "info":
		runInfo(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Run 'upkg help' for usage.")
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("upkg - Universal Package Manager")
	fmt.Println()
	fmt.Println("Usage: upkg <command> [arguments] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install, i   Download and install a package")
	fmt.Println("  search, s    Search for packages")
	fmt.Println("  info         Show package details")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  upkg install curl")
	fmt.Println("  upkg install git -b pacman")
	fmt.Println("  upkg search python --backend nix")
	fmt.Println()
}

// runInstall handles the 'install' subcommand
func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)

	var (
		cf          commonFlags
		pkgVersion  string
		platform    string
		noExtract   bool
		keepArchive bool
		noVerify    bool
	)

	// Register flags
	registerCommonFlags(fs, &cf)
	fs.StringVar(&pkgVersion, "version", "", "Package version")
	fs.StringVar(&pkgVersion, "v", "", "Package version (short)")
	fs.StringVar(&platform, "platform", "", "Target platform/architecture")
	fs.StringVar(&platform, "p", "", "Target platform (short)")
	fs.BoolVar(&noExtract, "no-extract", false, "Download only, don't extract")
	fs.BoolVar(&keepArchive, "keep-archive", false, "Keep archive file after extraction")
	fs.BoolVar(&noVerify, "no-verify", false, "Skip hash verification")

	fs.Parse(args)

	// Get positional argument (package name)
	pkgName := fs.Arg(0)
	if pkgName == "" {
		fmt.Println("Error: Package name required")
		fmt.Println("Usage: upkg install <package_name> [options]")
		os.Exit(1)
	}

	// FIX: Capture both manager and config
	mgr, config := setupManager(cf)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Prepare options
	pkg := &upkg.Package{
		Name:    pkgName,
		Version: pkgVersion,
	}

	opts := &upkg.DownloadOptions{
		Platform: platform,
	}

	if noExtract {
		f := false
		opts.Extract = &f
	}
	if keepArchive {
		t := true
		opts.KeepArchive = &t
	}
	if noVerify {
		f := false
		opts.VerifyHash = &f
	}

	// FIX: Use 'config.InstallPath' instead of 'mgr.Config.InstallPath'
	printHeader("Downloading Package", pkgName, pkgVersion, mgr.Backend(), config.InstallPath)

	err := mgr.Download(ctx, pkg, opts)
	if err != nil {
		fmt.Printf("\n❌ Error downloading package: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Successfully downloaded and installed %s\n", pkgName)
	
	// FIX: Use 'config.InstallPath' here as well
	printPostInstallInstructions(mgr.Backend(), config.InstallPath, pkgName)
}

// runSearch handles the 'search' subcommand
func runSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	var cf commonFlags
	registerCommonFlags(fs, &cf)
	fs.Parse(args)

	query := fs.Arg(0)
	if query == "" {
		fmt.Println("Error: Search query required")
		fmt.Println("Usage: upkg search <query> [options]")
		os.Exit(1)
	}

	// We ignore config here as we don't need it for search
	mgr, _ := setupManager(cf)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("Searching for '%s' using %s...\n\n", query, mgr.Backend())
	results, err := mgr.Search(ctx, query)
	if err != nil {
		fmt.Printf("Error searching packages: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No packages found")
		os.Exit(0)
	}

	printSearchResults(results)
}

// runInfo handles the 'info' subcommand
func runInfo(args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	var cf commonFlags
	registerCommonFlags(fs, &cf)
	fs.Parse(args)

	pkgName := fs.Arg(0)
	if pkgName == "" {
		fmt.Println("Error: Package name required")
		fmt.Println("Usage: upkg info <package_name> [options]")
		os.Exit(1)
	}

	mgr, _ := setupManager(cf)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pkgInfo, err := mgr.GetInfo(ctx, pkgName)
	if err != nil {
		fmt.Printf("Error getting package info: %v\n", err)
		os.Exit(1)
	}

	printPackageInfo(pkgInfo)
}

// registerCommonFlags binds flags to variables, supporting both short (-b) and long (--backend)
func registerCommonFlags(fs *flag.FlagSet, cf *commonFlags) {
	// We use a custom function for backend to update the struct
	fs.Func("backend", "Backend to use (auto, nix, brew, etc)", func(s string) error {
		cf.backendName = s
		return nil
	})
	fs.Func("b", "Backend (short)", func(s string) error {
		cf.backendName = s
		return nil
	})

	// Default backend if not set
	cf.backendName = "auto"

	fs.BoolVar(&cf.debug, "debug", false, "Enable debug logging")
	fs.StringVar(&cf.installPath, "install-path", "", "Custom install path (default: ~/.upkg)")
}

// setupManager initializes the upkg Manager based on flags
// FIX: Return both *upkg.Manager AND *upkg.Config so caller can access paths
func setupManager(cf commonFlags) (*upkg.Manager, *upkg.Config) {
	// 1. Load default configuration (which sets ~/.upkg and ~/.cache/upkg)
	config := upkg.DefaultConfig()

	// 2. Apply flags
	config.Debug = cf.debug
	if cf.debug {
		config.Logger = log.New(os.Stdout, "[upkg] ", log.LstdFlags)
	}
	if cf.installPath != "" {
		config.InstallPath = cf.installPath
	}

	// 3. Ensure Directories Exist
	// This handles the "~/.upkg/ if not exist auto creates" requirement
	if err := os.MkdirAll(config.InstallPath, 0755); err != nil {
		fmt.Printf("❌ Error creating install directory (%s): %v\n", config.InstallPath, err)
		os.Exit(1)
	}
	// Also ensure cache exists
	if err := os.MkdirAll(config.CachePath, 0755); err != nil {
		if cf.debug {
			fmt.Printf("Warning: Error creating cache directory (%s): %v\n", config.CachePath, err)
		}
	}

	// 4. Select Backend
	var backendType upkg.BackendType
	switch cf.backendName {
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
	case "zypper":
		backendType = upkg.BackendZypper
	default:
		fmt.Printf("Unknown backend: %s\n", cf.backendName)
		os.Exit(1)
	}

	mgr, err := upkg.NewManager(backendType, config)
	if err != nil {
		fmt.Printf("Error initializing manager: %v\n", err)
		os.Exit(1)
	}

	// FIX: Return the config we created so we can access InstallPath later
	return mgr, config
}

// ---------------------------------------------------------
// UI / Printing Helper Functions
// ---------------------------------------------------------

func printHeader(title, name, version, backend, path string) {
	if path == "" {
		path = "default"
	}
	fmt.Printf("\n")
	fmt.Printf("╔═══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ %-61s ║\n", center(title, 61))
	fmt.Printf("╠═══════════════════════════════════════════════════════════════╣\n")
	fmt.Printf("║ Package:  %-51s ║\n", truncate(name, 51))
	if version != "" {
		fmt.Printf("║ Version:  %-51s ║\n", truncate(version, 51))
	}
	fmt.Printf("║ Backend:  %-51s ║\n", truncate(backend, 51))
	fmt.Printf("║ Install:  %-51s ║\n", truncate(path, 51))
	fmt.Printf("╚═══════════════════════════════════════════════════════════════╝\n")
	fmt.Println()
}

func printSearchResults(results []*upkg.PackageInfo) {
	fmt.Printf("Found %d package(s):\n\n", len(results))
	for i, pkg := range results {
		if i >= 20 {
			fmt.Printf("... and %d more results (showing first 20)\n", len(results)-20)
			break
		}
		fmt.Printf("📦 %s (%s)\n", pkg.Name, pkg.Version)
		if pkg.Description != "" {
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
}

func printPackageInfo(pkgInfo *upkg.PackageInfo) {
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
}

func printPostInstallInstructions(backendName, installPath, pkgName string) {
	switch backendName {
	case "brew", "dpkg", "apt", "dnf", "apk", "pacman", "zypper":
		fmt.Printf("\n📍 Installation location: %s\n", installPath)
		fmt.Printf("\n💡 You may need to add the following to your PATH:\n")
		fmt.Printf("   export PATH=\"%s/bin:$PATH\"\n", installPath)
		fmt.Printf("   export PATH=\"%s/usr/bin:$PATH\"\n", installPath)
	case "choco":
		fmt.Printf("\n📍 Installation location: %s\n", installPath)
		fmt.Printf("\n💡 Windows binaries are usually in the tools directory or extracted root.\n")
	case "nix":
		fmt.Printf("\n📍 Nix package installed. Check your Nix profile for binaries.\n")
	}

	fmt.Printf("\n📦 Installed files can be found in:\n")
	switch backendName {
	case "dpkg", "apt", "dnf", "pacman", "zypper":
		fmt.Printf("   %s/usr/bin/     - Executables\n", installPath)
		fmt.Printf("   %s/usr/lib/     - Libraries\n", installPath)
	case "apk":
		fmt.Printf("   %s/bin/         - Executables\n", installPath)
		fmt.Printf("   %s/lib/         - Libraries\n", installPath)
	case "brew":
		fmt.Printf("   %s/Cellar/      - Bottle files\n", installPath)
	case "choco":
		fmt.Printf("   %s/%s/          - Package content\n", installPath, pkgName)
	}
}

// Utility functions

func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func center(s string, width int) string {
	if len(s) >= width {
		return s
	}
	padding := (width - len(s)) / 2
	return strings.Repeat(" ", padding) + s
}

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
	for i := range lines {
		if len(lines[i]) < width {
			lines[i] += strings.Repeat(" ", width-len(lines[i]))
		}
	}
	return lines
}