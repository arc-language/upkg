// pkg/env/constants.go
package env

import (
    "path/filepath"
    "runtime"
)

// GetPackageLayout returns the typical directory structure for packages from each backend
// These are RELATIVE paths within the extracted package, not absolute system paths
func GetPackageLayout(backend string) PackageLayout {
    switch backend {
    case "apt", "dpkg":
        return getDebianLayout()
    case "dnf", "yum":
        return getFedoraLayout()
    case "brew":
        return getBrewLayout()
    case "pacman":
        return getArchLayout()
    case "apk":
        return getAlpineLayout()
    case "zypper":
        return getOpenSUSELayout()
    case "choco":
        return getChocoLayout()
    case "nix":
        return getNixLayout()
    default:
        return getDefaultLayout()
    }
}

// Debian/Ubuntu packages extract with full /usr hierarchy
func getDebianLayout() PackageLayout {
    arch := runtime.GOARCH
    if arch == "amd64" {
        arch = "x86_64"
    }
    
    return PackageLayout{
        // .deb packages contain: usr/lib/x86_64-linux-gnu/libssl.so.3
        Libraries: []string{
            filepath.Join("usr", "lib", arch+"-linux-gnu"),
            filepath.Join("usr", "lib"),
            filepath.Join("lib", arch+"-linux-gnu"),
            filepath.Join("lib"),
            filepath.Join("usr", "local", "lib"),
        },
        // .deb packages contain: usr/include/openssl/ssl.h
        Includes: []string{
            filepath.Join("usr", "include"),
            filepath.Join("usr", "local", "include"),
        },
        // .deb packages contain: usr/lib/x86_64-linux-gnu/pkgconfig/openssl.pc
        PkgConfig: []string{
            filepath.Join("usr", "lib", arch+"-linux-gnu", "pkgconfig"),
            filepath.Join("usr", "lib", "pkgconfig"),
            filepath.Join("usr", "share", "pkgconfig"),
        },
        // .deb packages contain: usr/bin/openssl
        Binaries: []string{
            filepath.Join("usr", "bin"),
            filepath.Join("usr", "local", "bin"),
            filepath.Join("bin"),
        },
    }
}

// Fedora/RHEL packages use lib64 for 64-bit
func getFedoraLayout() PackageLayout {
    return PackageLayout{
        // .rpm packages contain: usr/lib64/libssl.so.3
        Libraries: []string{
            filepath.Join("usr", "lib64"),
            filepath.Join("usr", "lib"),
            filepath.Join("lib64"),
            filepath.Join("lib"),
        },
        Includes: []string{
            filepath.Join("usr", "include"),
        },
        PkgConfig: []string{
            filepath.Join("usr", "lib64", "pkgconfig"),
            filepath.Join("usr", "lib", "pkgconfig"),
            filepath.Join("usr", "share", "pkgconfig"),
        },
        Binaries: []string{
            filepath.Join("usr", "bin"),
            filepath.Join("bin"),
        },
    }
}

// Homebrew uses a flat structure when extracted
func getBrewLayout() PackageLayout {
    // Homebrew bottles extract to a simple structure
    // bottle contents: lib/libssl.dylib, include/openssl/
    return PackageLayout{
        Libraries: []string{
            "lib",
        },
        Includes: []string{
            "include",
        },
        PkgConfig: []string{
            filepath.Join("lib", "pkgconfig"),
        },
        Binaries: []string{
            "bin",
        },
    }
}

// Arch Linux packages use simple /usr structure
func getArchLayout() PackageLayout {
    return PackageLayout{
        // Pacman packages contain: usr/lib/libssl.so
        Libraries: []string{
            filepath.Join("usr", "lib"),
            filepath.Join("lib"),
        },
        Includes: []string{
            filepath.Join("usr", "include"),
        },
        PkgConfig: []string{
            filepath.Join("usr", "lib", "pkgconfig"),
            filepath.Join("usr", "share", "pkgconfig"),
        },
        Binaries: []string{
            filepath.Join("usr", "bin"),
            filepath.Join("bin"),
        },
    }
}

// Alpine Linux uses simple /usr structure (like Arch)
func getAlpineLayout() PackageLayout {
    return PackageLayout{
        Libraries: []string{
            filepath.Join("usr", "lib"),
            filepath.Join("lib"),
        },
        Includes: []string{
            filepath.Join("usr", "include"),
        },
        PkgConfig: []string{
            filepath.Join("usr", "lib", "pkgconfig"),
            filepath.Join("usr", "share", "pkgconfig"),
        },
        Binaries: []string{
            filepath.Join("usr", "bin"),
            filepath.Join("bin"),
        },
    }
}

// OpenSUSE uses lib64 like Fedora
func getOpenSUSELayout() PackageLayout {
    return PackageLayout{
        Libraries: []string{
            filepath.Join("usr", "lib64"),
            filepath.Join("usr", "lib"),
            filepath.Join("lib64"),
            filepath.Join("lib"),
        },
        Includes: []string{
            filepath.Join("usr", "include"),
        },
        PkgConfig: []string{
            filepath.Join("usr", "lib64", "pkgconfig"),
            filepath.Join("usr", "lib", "pkgconfig"),
            filepath.Join("usr", "share", "pkgconfig"),
        },
        Binaries: []string{
            filepath.Join("usr", "bin"),
            filepath.Join("bin"),
        },
    }
}

// Chocolatey packages are inconsistent, use common patterns
func getChocoLayout() PackageLayout {
    return PackageLayout{
        // Chocolatey packages vary wildly, these are common patterns
        Libraries: []string{
            "lib",
            filepath.Join("tools", "lib"),
            "bin", // DLLs often in bin/
        },
        Includes: []string{
            "include",
            filepath.Join("tools", "include"),
        },
        PkgConfig: []string{
            filepath.Join("lib", "pkgconfig"),
        },
        Binaries: []string{
            "bin",
            "tools",
        },
    }
}

// Nix packages use a flat structure when extracted
func getNixLayout() PackageLayout {
    return PackageLayout{
        // Nix store paths extract to: lib/, include/, bin/
        Libraries: []string{
            "lib",
            "lib64",
        },
        Includes: []string{
            "include",
        },
        PkgConfig: []string{
            filepath.Join("lib", "pkgconfig"),
        },
        Binaries: []string{
            "bin",
        },
    }
}

// Default layout for unknown backends (FHS-like)
func getDefaultLayout() PackageLayout {
    return PackageLayout{
        Libraries: []string{
            filepath.Join("usr", "lib"),
            filepath.Join("lib"),
        },
        Includes: []string{
            filepath.Join("usr", "include"),
        },
        PkgConfig: []string{
            filepath.Join("usr", "lib", "pkgconfig"),
        },
        Binaries: []string{
            filepath.Join("usr", "bin"),
            filepath.Join("bin"),
        },
    }
}

// GetLibraryExtensions returns file extensions to look for based on OS
func GetLibraryExtensions() []string {
    switch runtime.GOOS {
    case "darwin":
        return []string{".dylib", ".a"}
    case "windows":
        return []string{".dll", ".lib"}
    default: // linux, etc.
        return []string{".so", ".a"}
    }
}

// GetSharedLibraryExtensions returns only shared library extensions
func GetSharedLibraryExtensions() []string {
    switch runtime.GOOS {
    case "darwin":
        return []string{".dylib"}
    case "windows":
        return []string{".dll"}
    default:
        return []string{".so"}
    }
}

// GetStaticLibraryExtensions returns only static library extensions
func GetStaticLibraryExtensions() []string {
    switch runtime.GOOS {
    case "windows":
        return []string{".lib"} // Can be import lib or static lib
    default:
        return []string{".a"}
    }
}