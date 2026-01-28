<h1 align="center">
  <img src="./.github/upkg_logo.png" alt="upkg" width="200px">
</h1>

<h4 align="center">Universal Package Manager<br>One API. Multiple backends.</h4>

<p align="center">
    <img src="https://img.shields.io/badge/Version-1.0-blue" alt="Version">
    <img src="https://img.shields.io/badge/Backends-Nix%20%7C%20Homebrew-purple" alt="Backends">
    <img src="https://img.shields.io/badge/License-MIT%20%7C%20Apache--2.0-green" alt="License">
</p>

---

## What is upkg?

upkg lets you **download and manage packages from any package manager—Nix, Homebrew, or future backends—from a single unified API.** No more learning different CLIs and APIs.

Write your automation once. Switch backends freely. With full control when you need it.

---

## Write Once. Run Anywhere.

```go
package main

import (
    "context"
    "fmt"
    "github.com/arc-language/upkg"
)

func main() {
    // Auto-detect best backend for your system
    mgr, _ := upkg.NewManager(upkg.BackendAuto, nil)
    defer mgr.Close()
    
    ctx := context.Background()
    
    // Download wget - same API regardless of backend
    pkg := &upkg.Package{
        Name:    "wget",
        Version: "1.21.3",
    }
    
    mgr.Download(ctx, pkg, &upkg.DownloadOptions{})
    
    // Get package info
    info, _ := mgr.GetInfo(ctx, "gcc")
    fmt.Printf("GCC %s: %s\n", info.Version, info.Description)
}
```

**That's it.** Same Go API. Different backends. Zero friction.

---

## Core Features

### Multi-Backend Support
Switch backends with a single parameter change:

```go
// Use Homebrew
mgr, _ := upkg.NewManager(upkg.BackendBrew, nil)

// Use Nix
mgr, _ := upkg.NewManager(upkg.BackendNix, nil)

// Auto-detect (prefers Brew on macOS, Nix on Linux)
mgr, _ := upkg.NewManager(upkg.BackendAuto, nil)
```

### Unified Package Operations
```go
import "github.com/arc-language/upkg"

// Download packages
pkg := &upkg.Package{Name: "curl"}
mgr.Download(ctx, pkg, &upkg.DownloadOptions{
    Extract:     true,
    VerifyHash:  true,
    KeepArchive: false,
})

// Get package information
info, _ := mgr.GetInfo(ctx, "python")
fmt.Printf("Available on: %v\n", info.Platforms)

// Search packages (backend-dependent)
results, _ := mgr.Search(ctx, "compiler")
```

### Flexible Configuration
```go
config := &upkg.Config{
    InstallPath: "/custom/path",
    CachePath:   "/tmp/cache",
    Debug:       true,
    Timeout:     5 * time.Minute,
}

// Backend-specific settings
config.Nix = &upkg.NixConfig{
    CacheURL: "https://cache.nixos.org",
}

config.Brew = &upkg.BrewConfig{
    APIURL:      "https://formulae.brew.sh/api",
    RegistryURL: "https://ghcr.io/v2/homebrew/core",
}

mgr, _ := upkg.NewManager(upkg.BackendAuto, config)
```

### Backend-Specific Features
Access advanced features when needed:

```go
// Nix: Download multiple outputs (bin, dev, lib)
pkg := &upkg.Package{
    Name:   "gcc",
    Output: "dev", // Get development files
}

// Homebrew: Target specific platforms
opts := &upkg.DownloadOptions{
    Platform: "arm64_sonoma", // macOS 14 ARM64
}
```

### CLI Tool
```bash
# Download wget using auto-detected backend
upkg -package=wget

# Use specific backend
upkg -backend=nix -package=gcc -version=13.2.0

# Get package info
upkg -package=python -info

# Enable debug output
upkg -package=curl -debug

# Custom install path
upkg -package=node -install-path=/opt/packages
```

---

## Supported Backends

| Backend | Status | Package Count | Platforms |
|:---|:---:|:---:|:---|
| **Nix** | ✅ Stable | 80,000+ | Linux, macOS |
| **Homebrew** | ✅ Stable | 6,000+ | macOS, Linux |
| **APT** | 🚧 Planned | - | Debian, Ubuntu |
| **DNF/YUM** | 🚧 Planned | - | Fedora, RHEL |
| **Pacman** | 🚧 Planned | - | Arch Linux |
| **Chocolatey** | 🚧 Planned | - | Windows |

---

## Getting Started

### Installation

```bash
go get github.com/arc-language/upkg
```

### Library Usage

```go
package main

import (
    "context"
    "log"
    "github.com/arc-language/upkg"
)

func main() {
    // Create manager with auto backend detection
    mgr, err := upkg.NewManager(upkg.BackendAuto, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer mgr.Close()
    
    ctx := context.Background()
    
    // Download a package
    pkg := &upkg.Package{Name: "wget"}
    err = mgr.Download(ctx, pkg, &upkg.DownloadOptions{})
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Successfully installed wget using %s backend", mgr.Backend())
}
```

### CLI Usage

Build and install the CLI:

```bash
cd cmd
go build -o upkg
sudo mv upkg /usr/local/bin/

# Use it
upkg -package=wget
upkg -backend=nix -package=gcc -info
```

**No external dependencies** beyond the package managers themselves. If you have Nix or Homebrew installed, upkg will use them.

---

## Why upkg?

| Feature | Direct Nix | Direct Brew | upkg |
|:---|:---:|:---:|:---:|
| **Unified API** | ❌ | ❌ | ✅ |
| **Switch Backends** | ❌ | ❌ | ✅ |
| **Go Integration** | ⚠️ (Shell) | ⚠️ (Shell) | ✅ |
| **Type Safety** | ❌ | ❌ | ✅ |
| **Cross-Platform** | ✅ | ✅ | ✅ |
| **Package Verification** | ✅ | ✅ | ✅ |
| **Programmatic Control** | ⚠️ (Complex) | ⚠️ (Complex) | ✅ |

---

## Project Structure

```
upkg/
├── upkg.go              # Main manager implementation
├── types.go             # Public types (re-exports)
├── errors.go            # Error types
├── pkg/
│   ├── backend/
│   │   ├── types.go     # Backend interface & types
│   │   ├── nix.go       # Nix backend implementation
│   │   └── brew.go      # Homebrew backend implementation
│   ├── nix/             # Nix-specific package manager
│   │   ├── manager.go
│   │   ├── types.go
│   │   ├── platform.go
│   │   ├── parser.go
│   │   ├── hash.go
│   │   ├── client.go
│   │   └── constants.go
│   └── brew/            # Homebrew-specific package manager
│       ├── manager.go
│       ├── types.go
│       ├── platform.go
│       ├── client.go
│       └── constants.go
└── cmd/
    └── main.go          # CLI application
```

---

## Learn More

- **[API Documentation](docs/api.md)** - Complete Go API reference
- **[Backend Guide](docs/backends.md)** - Understanding different backends
- **[Configuration](docs/config.md)** - Advanced configuration options
- **[CLI Reference](docs/cli.md)** - Command-line usage
- **[Examples](examples/)** - Sample programs and use cases
- **[Contributing](CONTRIBUTING.md)** - How to add new backends

---

## Roadmap

- [x] Nix backend with full output support
- [x] Homebrew backend with OCI registry
- [x] Auto-detection of best backend
- [x] Package verification (SHA256)
- [x] Platform-specific downloads
- [ ] Search functionality across backends
- [ ] Package caching and deduplication
- [ ] APT backend (Debian/Ubuntu)
- [ ] DNF/YUM backend (Fedora/RHEL)
- [ ] Pacman backend (Arch Linux)
- [ ] Chocolatey backend (Windows)
- [ ] Package dependency resolution
- [ ] Version constraint support
- [ ] Parallel downloads
- [ ] Progress reporting

---

## License

Licensed under either of

*   Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
*   MIT license ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

at your option.

## Contribution

Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in the work by you, as defined in the Apache-2.0 license, shall be
dual licensed as above, without any additional terms or conditions.