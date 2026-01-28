// internal/cli/root.go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/arc-language/upkg/pkg/core"
)

var (
	cfgFile string
	backend string
	debug   bool
	config  *core.Config
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "upkg",
	Short: "Universal Package Manager",
	Long: `upkg - Universal Package Manager
	
A unified package manager that works across Nix, Homebrew, and more.
Simplify package management across different systems with one tool.`,
	Version: "0.1.0",
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/upkg/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&backend, "backend", "", "package manager backend to use (nix, brew)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	// Add commands
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	var err error
	config, err = core.LoadConfig(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		config = core.DefaultConfig()
	}

	// Override config with flags
	if backend != "" {
		config.DefaultBackend = backend
	}
	if debug {
		config.Debug = true
	}
}