// cmd/upkg/main.go
package main

import (
	"fmt"
	"os"

	"github.com/arc-language/upkg/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}