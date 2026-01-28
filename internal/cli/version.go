// internal/cli/version.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("upkg version 0.1.0")
		fmt.Println("Universal Package Manager")
		fmt.Println("https://github.com/arc-language/upkg")
	},
}