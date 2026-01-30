// pkg/env/doc.go
package env

/*
Package env provides environment management for upkg package installations.

It handles:
  - Discovering library and include paths from extracted packages
  - Generating compiler and linker flags
  - Finding specific libraries within installations
  - Creating activation scripts for shell environments

Basic Usage:

    import "github.com/arc-language/upkg/pkg/env"

    // Create environment
    env := env.New("/opt/upkg", "apt")

    // Get paths
    libs := env.GetLibraryPaths()
    includes := env.GetIncludePaths()

    // Find specific library
    ssl := env.FindLibrary("ssl")
    if ssl != nil {
        fmt.Printf("Found: %s at %s\n", ssl.Name, ssl.Path)
    }

    // Get compiler flags
    flags := env.GetCompilerFlags()
    for _, flag := range flags.IncludeFlags {
        fmt.Println(flag) // -I/opt/upkg/usr/include
    }

Backend Layouts:

Each backend (apt, brew, nix, etc.) has different directory structures
when packages are extracted. This package knows about these layouts and
searches in the appropriate locations for each backend type.

For example, APT packages extract to usr/lib/x86_64-linux-gnu/ while
Homebrew packages extract to lib/ directly.
*/