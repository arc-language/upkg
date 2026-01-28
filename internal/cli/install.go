// internal/cli/install.go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/arc-language/upkg/pkg/core"
	"github.com/arc-language/upkg/pkg/platform"
	"github.com/arc-language/upkg/pkg/registry"
)

var (
	installVersion string
	installPlatform string
)

var installCmd = &cobra.Command{
	Use:   "install [package...]",
	Short: "Install one or more packages",
	Long: `Install packages using the configured or auto-detected backend.
	
Examples:
  upkg install wget
  upkg install wget --backend=nix
  upkg install nginx --version=1.24.0
  upkg install python3 nodejs golang`,
	Args: cobra.MinimumNArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installVersion, "version", "", "specific version to install")
	installCmd.Flags().StringVar(&installPlatform, "platform", "", "target platform")
}

func runInstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Detect platform
	plat, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("detecting platform: %w", err)
	}

	if config.Debug {
		fmt.Printf("Platform: %s\n", plat)
		fmt.Printf("Available backends: %v\n", plat.Available)
	}

	// Resolve backend
	var backendName string
	if config.DefaultBackend != "" {
		backendName = config.DefaultBackend
	} else {
		backendName = plat.Preferred
	}

	if backendName == "" {
		return fmt.Errorf("no package manager available")
	}

	pm, err := registry.Get(backendName, config.InstallPath, config.Debug)
	if err != nil {
		return fmt.Errorf("initializing backend: %w", err)
	}

	fmt.Printf("Using backend: %s\n", pm.Name())

	// Install each package
	for _, pkg := range args {
		fmt.Printf("\nInstalling %s...\n", pkg)

		opts := &core.InstallOptions{
			Version:     installVersion,
			Platform:    installPlatform,
			Extract:     true,
			VerifyHash:  true,
			KeepArchive: false,
		}

		if err := pm.Install(ctx, pkg, opts); err != nil {
			fmt.Fprintf(os.Stderr, "✗ Failed to install %s: %v\n", pkg, err)
			continue
		}

		fmt.Printf("✓ Successfully installed %s\n", pkg)
	}

	return nil
}