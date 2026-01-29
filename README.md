<h1 align="center">
  <img src="./.github/upkg_logo.jpg" alt="upkg" width="200px">
</h1>

<h4 align="center">Universal Package Manager<br>One API. Multiple backends.</h4>

<p align="center">
    <img src="https://img.shields.io/badge/Version-1.0-blue" alt="Version">
    <img src="https://img.shields.io/badge/Backends-Nix%20%7C%20Brew%20%7C%20Apt%20%7C%20Dnf%20%7C%20Choco-purple" alt="Backends">
    <img src="https://img.shields.io/badge/License-MIT%20%7C%20Apache--2.0-green" alt="License">
</p>

---

## Quick Start

```bash
# Install curl using Ubuntu/Debian Apt
upkg install curl -b apt
```

**upkg** is a unified interface for package management. It abstracts away the differences between `apt`, `dnf`, `brew`, `nix`, `choco`, and more, giving you a single CLI and a single Go API to manage software on any operating system.

---

## Installation & Building

To build the CLI tool from source:

```bash
# 1. Clone the repository
git clone https://github.com/arc-language/upkg
cd upkg

# 2. Build the CLI
cd cli
go build -o upkg main.go

# 3. Move to path (optional)
sudo mv upkg /usr/local/bin/
```

Now you are ready to run:
```bash
upkg help
```

---

## CLI Usage

The CLI uses a subcommand structure (`install`, `search`, `info`). It attempts to auto-detect your operating system's preferred package manager if you don't specify one.

### 1. Install Packages
```bash
# Auto-detect backend (Homebrew on Mac, Apt on Ubuntu, Choco on Windows, etc.)
upkg install wget

# Force a specific backend
upkg install git -b pacman
upkg install nodejs --backend=choco
upkg install ffmpeg -b nix

# Install specific version
upkg install python -v 3.11
```

### 2. Search Packages
```bash
# Search using the active backend
upkg search python

# Search a specific backend's repository
upkg search nginx -b brew
```

### 3. Get Package Info
```bash
# View metadata (version, license, description) without installing
upkg info gcc
```

---

## Library Usage (for Go Developers)

**upkg** is primarily designed as a library to power automation tools, IDEs, and cross-platform setup scripts.

### Import
```go
import "github.com/arc-language/upkg"
```

### Example: Auto-detect and Download
```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/arc-language/upkg"
)

func main() {
    // 1. Create a Manager
    // BackendAuto will check for Apt, Dnf, Brew, Choco, Nix, etc.
    config := upkg.DefaultConfig()
    mgr, err := upkg.NewManager(upkg.BackendAuto, config)
    if err != nil {
        log.Fatal(err)
    }
    defer mgr.Close()

    fmt.Printf("Using backend: %s\n", mgr.Backend())

    // 2. Define the package
    pkg := &upkg.Package{
        Name: "ripgrep",
    }
    
    // 3. Download/Install
    // Options allow you to control extraction, verification, etc.
    opts := &upkg.DownloadOptions{
        VerifyHash:  nil, // defaults to true
    }

    ctx := context.Background()
    if err := mgr.Download(ctx, pkg, opts); err != nil {
        log.Fatalf("Failed to install: %v", err)
    }

    fmt.Println("Successfully installed ripgrep!")
}
```

### Example: Backend-Specific Configuration
You can pass custom configuration if you need to tweak paths or backend URLs.

```go
config := upkg.DefaultConfig()
config.InstallPath = "/opt/my-app/deps"

// Configure Nix specifically
config.Nix = &upkg.NixConfig{
    CacheURL: "https://cache.nixos.org",
}

mgr, _ := upkg.NewManager(upkg.BackendNix, config)
```

---

## Supported Backends

| Backend | Flag | System | Status |
|:---|:---:|:---|:---:|
| **Nix** | `nix` | Linux / macOS | ✅ Stable |
| **Homebrew** | `brew` | macOS / Linux | ✅ Stable |
| **APT** | `apt` | Ubuntu / Debian | ✅ Stable |
| **DPKG** | `dpkg` | Debian | ✅ Stable |
| **DNF** | `dnf` | Fedora / RHEL | ✅ Stable |
| **APK** | `apk` | Alpine Linux | ✅ Stable |
| **Pacman** | `pacman` | Arch Linux | ✅ Stable |
| **Zypper** | `zypper` | OpenSUSE | ✅ Stable |
| **Chocolatey** | `choco` | Windows | ✅ Stable |

---

## Project Structure

```
upkg/
├── upkg.go              # Main entry point & Manager logic
├── types.go             # Shared types (Package, PackageInfo)
├── cmd/
│   └── main.go          # CLI entry point
└── pkg/
    ├── backend/         # Backend implementations
    │   ├── apt.go       # Ubuntu/Debian logic
    │   ├── brew.go      # Homebrew logic
    │   ├── choco.go     # Windows Chocolatey logic
    │   ├── nix.go       # Nix logic
    │   └── ...          # (apk, dnf, dpkg, pacman, zypper)
    └── ...              # Internal package manager drivers
```

## Contributing

1. Fork the repo
2. Create a feature branch
3. Implement the `Backend` interface in `pkg/backend/`
4. Submit a Pull Request

## License

Licensed under the MIT License.