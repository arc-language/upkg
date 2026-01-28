// internal/cli/info.go
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/arc-language/upkg/pkg/platform"
	"github.com/arc-language/upkg/pkg/registry"
)

var infoCmd = &cobra.Command{
	Use:   "info [package]",
	Short: "Show information about a package",
	Long:  `Display detailed information about a package from the configured backend.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	pkg := args[0]

	// Detect platform
	plat, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("detecting platform: %w", err)
	}

	// Resolve backend
	var backendName string
	if config.DefaultBackend != "" {
		backendName = config.DefaultBackend
	} else {
		backendName = plat.Preferred
	}

	pm, err := registry.Get(backendName, config.InstallPath, config.Debug)
	if err != nil {
		return fmt.Errorf("initializing backend: %w", err)
	}

	// Get package info
	info, err := pm.Info(ctx, pkg)
	if err != nil {
		return fmt.Errorf("getting package info: %w", err)
	}

	// Display info
	fmt.Printf("Package: %s\n", info.Name)
	fmt.Printf("Version: %s\n", info.Version)
	fmt.Printf("Backend: %s\n", info.Backend)
	fmt.Printf("Platform: %s\n", info.Platform)
	if info.Description != "" {
		fmt.Printf("Description: %s\n", info.Description)
	}

	return nil
}