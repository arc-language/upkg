// pkg/env/types.go
package env

// PackageLayout defines where files are located within an extracted package
type PackageLayout struct {
    Libraries []string // Relative paths to library directories
    Includes  []string // Relative paths to include directories
    PkgConfig []string // Relative paths to pkg-config directories
    Binaries  []string // Relative paths to binary directories
}

// Library represents a found library file
type Library struct {
    Name     string // Library name (e.g., "ssl")
    Path     string // Absolute path to library file
    Type     string // Extension: ".so", ".a", ".dylib", ".dll", ".lib"
    Version  string // Version if detected (e.g., "3" from libssl.so.3)
    IsStatic bool   // True for .a files
}

// Environment represents a package installation environment
type Environment struct {
    InstallPath string // Root installation path (e.g., /opt/upkg)
    BackendType string // Backend type (apt, brew, nix, etc.)
}

// CompilerFlags holds compiler and linker flags
type CompilerFlags struct {
    IncludeFlags []string // -I flags
    LibraryFlags []string // -L flags
    LinkFlags    []string // -l flags
}