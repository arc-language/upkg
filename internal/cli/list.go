// internal/cli/list.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/arc-language/upkg/pkg/platform"
	"github.com/arc-language/upkg/pkg/registry"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available package managers",
	Long:  `List all available package manager backends on this system.`,
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	// Detect platform
	plat, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("detecting platform: %w", err)
	}

	fmt.Printf("Platform: %s/%s\n\n", plat.OS, plat.Arch)
	fmt.Printf("Available backends:\n")
	for _, backend := range plat.Available {
		marker := " "
		if backend == plat.Preferred {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, backend)
	}

	if plat.Preferred != "" {
		fmt.Printf("\n* = preferred backend\n")
	}

	fmt.Printf("\nRegistered backends: %v\n", registry.Available())

	return nil
}