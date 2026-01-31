<h1 align="center">
  <img src="./.github/upkg_logo.jpg" alt="upkg" width="200px">
</h1>

<h4 align="center">Universal Package Manager<br>One API. Multiple backends.</h4>

<p align="center">
    <img src="https://img.shields.io/badge/Version-1.0-blue" alt="Version">
    <img src="https://img.shields.io/badge/Backends-Nix%20%7C%20Winget%20%7C%20Brew%20%7C%20Apt%20%7C%20Dnf%20%7C%20Choco-purple" alt="Backends">
    <img src="https://img.shields.io/badge/License-MIT%20%7C%20Apache--2.0-green" alt="License">
</p>

---

## Quick Start
```bash
# Create an isolated environment (auto mode, no backend needed)
upkg env create myproject

# Activate it
upkg env activate myproject
eval "$(upkg shell)"

# Install packages — upkg figures out the rest
upkg install curl git openssl

# Now commands just work!
curl --version
git --version
```

**upkg** is a unified interface for package management. It abstracts away the differences between `winget`, `nix`, `apt`, `dnf`, `brew`, `choco`, and more, giving you a single CLI and a single Go API to manage software on any operating system.

---

## Installation & Building

To build the CLI tool from source:
```bash
# 1. Clone the repository
git clone https://github.com/arc-language/upkg
cd upkg

# 2. Build the CLI
cd cmd/upkg
go build -o upkg main.go

# 3. Move to path (optional)
# Linux/macOS
sudo mv upkg /usr/local/bin/
# Windows (PowerShell)
mv upkg.exe C:\Windows\System32\
```

Now you are ready to run:
```bash
upkg help
```

---

## CLI Usage

### Environment Management

Create isolated package environments for different projects:
```bash
# Create a new environment (auto mode — detects backend for your system)
upkg env create myproject

# Or explicitly set a backend
upkg env create myproject --backend nix

# List all environments
upkg env list

# Activate an environment
upkg env activate myproject

# Get environment details
upkg env info myproject

# Deactivate current environment
upkg env deactivate

# Remove an environment
upkg env remove myproject
```

### Shell Integration

To use installed packages directly in your shell:
```bash
# After activating an environment
eval "$(upkg shell)"

# Now all binaries from the environment are in your PATH
gcc --version
python3 --version
```

Add to your `~/.bashrc` or `~/.zshrc` for automatic activation:
```bash
eval "$(upkg shell)"
```

### Package Management
```bash
# Install packages to active environment
upkg install gcc
upkg install wget python3

# Install multiple packages at once
upkg install git curl vim

# Search for packages
upkg search python

# Get package information
upkg info gcc

# List installed packages
upkg list

# Run commands in environment (without shell integration)
upkg run gcc myfile.c -o myfile
```

### Complete Workflow Example
```bash
# 1. Create environment for a C++ project (auto mode)
upkg env create cpp-dev
upkg env activate cpp-dev
eval "$(upkg shell)"

# 2. Install development tools
upkg install gcc cmake make

# 3. Work on your project
gcc myprogram.c -o myprogram
./myprogram

# 4. Create another environment for Python
upkg env create python-ml
upkg env activate python-ml
eval "$(upkg shell)"

# 5. Install Python tools
upkg install python3

# 6. Switch between environments
upkg env activate cpp-dev
upkg env activate python-ml

# 7. Clean up when done
upkg env remove cpp-dev
```

---

## Library Usage (for Go Developers)

**upkg** is primarily designed as a library to power automation tools, IDEs, and cross-platform setup scripts.

### Import
```go
import (
    "github.com/arc-language/upkg"
    "github.com/arc-language/upkg/pkg/env"
)
```

### Example: Environment Management
```go
package main

import (
    "context"
    "fmt"
    "log"
    "github.com/arc-language/upkg"
    "github.com/arc-language/upkg/pkg/env"
)

func main() {
    // 1. Create environment manager
    envMgr := env.NewEnvironmentManager("")
    
    // 2. Create a new environment (auto mode)
    envSpec, err := envMgr.CreateEnv("myproject", "auto")
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. Activate it
    envMgr.ActivateEnv("myproject")
    
    // 4. Install packages — auto mode resolves names via registry
    config := upkg.DefaultConfig()
    config.InstallPath = envSpec.InstallPath
    
    mgr, err := upkg.NewManager(upkg.BackendAuto, config)
    if err != nil {
        log.Fatal(err)
    }
    defer mgr.Close()
    
    pkg := &upkg.Package{Name: "openssl"}
    if err := mgr.Download(context.Background(), pkg, nil); err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Successfully installed openssl!")
}
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
    // BackendAuto detects native backend and resolves package names
    // via the registry automatically
    config := upkg.DefaultConfig()
    mgr, err := upkg.NewManager(upkg.BackendAuto, config)
    if err != nil {
        log.Fatal(err)
    }
    defer mgr.Close()

    fmt.Printf("Using backend: %s\n", mgr.Backend())

    // Use the friendly name — registry handles the rest
    pkg := &upkg.Package{
        Name: "curl",
    }
    
    opts := &upkg.DownloadOptions{
        VerifyHash: nil, // defaults to true
    }

    ctx := context.Background()
    if err := mgr.Download(ctx, pkg, opts); err != nil {
        log.Fatalf("Failed to install: %v", err)
    }

    fmt.Println("Successfully installed curl!")
}
```

### Example: Backend-Specific Configuration
```go
config := upkg.DefaultConfig()
config.InstallPath = "/opt/my-app/deps"

// Configure Nix specifically
config.Nix = &upkg.NixConfig{
    CacheURL: "https://cache.nixos.org",
}

mgr, _ := upkg.NewManager(upkg.BackendNix, config)
```

### Example: Working with Environments
```go
// Get active environment
envSpec, _ := envMgr.GetActiveEnv()

// Get environment configuration
environment := envSpec.GetEnvironment()

// Get library paths
libs := environment.GetLibraryPaths()
includes := environment.GetIncludePaths()

// Find specific library
ssl := environment.FindLibrary("ssl")
if ssl != nil {
    fmt.Printf("Found: %s at %s\n", ssl.Name, ssl.Path)
}

// Get compiler flags
flags := environment.GetCompilerFlags()
for _, flag := range flags.IncludeFlags {
    fmt.Println(flag) // -I/path/to/include
}
```

---

## Supported Backends

upkg supports a wide range of native package managers. Each backend knows how to download, extract, and install packages for its platform.

| Backend | Flag | System | Status |
|:---|:---:|:---|:---:|
| **Winget** | `winget` | Windows | ✅ Stable |
| **Nix** | `nix` | Linux / macOS | ✅ Stable |
| **Homebrew** | `brew` | macOS / Linux | ✅ Stable |
| **APT** | `apt` | Ubuntu / Debian | ✅ Stable |
| **DPKG** | `dpkg` | Debian | ✅ Stable |
| **DNF** | `dnf` | Fedora / RHEL | ✅ Stable |
| **APK** | `apk` | Alpine Linux | ✅ Stable |
| **Pacman** | `pacman` | Arch Linux | ✅ Stable |
| **Zypper** | `zypper` | OpenSUSE | ✅ Stable |
| **Chocolatey** | `choco` | Windows | ✅ Stable |

You can force a specific backend when creating an environment:
```bash
upkg env create myproject --backend nix
```

---

## Auto Mode

By default, upkg runs in **auto mode** — no backend flag needed. It does two things automatically:

**1. Detects your native backend.** When you run on Ubuntu it picks `dpkg`. On macOS it picks `brew`. On Windows it picks `winget`. On Arch it picks `pacman`. You never have to think about it.

**2. Resolves package names via the registry.** Every package manager on earth calls the same library something different. OpenSSL is `libssl-dev` on Debian, `openssl-devel` on Fedora, `openssl@3` on Homebrew, `OpenSSL.OpenSSL` on Winget. The registry maps one friendly name to all of them:

```
deps/
├── sqlite3/
│   └── index.toml      # apt = "libsqlite3-dev", brew = "sqlite", ...
├── openssl/
│   └── index.toml      # apt = "libssl-dev", brew = "openssl@3", ...
└── curl/
    └── index.toml      # apt = "libcurl4-openssl-dev", brew = "curl", ...
```

So you just write:
```bash
upkg install sqlite3
upkg install openssl
upkg install curl
```

And it works on any platform, no changes needed.

**As the registry grows**, it will cover more and more of the common ecosystem automatically — generic libraries, development headers, binary tools, and so on. Each new entry in `deps/` instantly makes that package available on every supported platform at once. Adding a package is as simple as adding a folder with one TOML file mapping the native names per backend.

---

## Features

- **Cross-platform**: Works on Linux, macOS, and Windows
- **Auto mode**: Detects backend and resolves package names automatically
- **Backend flexibility**: Optionally choose from winget, nix, apt, dnf, brew, and more
- **Registry-driven**: Friendly package names resolve to native names per platform
- **Isolated Environments**: Keep project dependencies separate
- **Index Syncing**: Automatically downloads package registry and indices on first run
- **Smart library detection**: Automatically finds libraries and headers
- **Compiler integration**: Generate flags for gcc, clang, etc.
- **No system pollution**: Install packages without affecting system

---

## Project Structure
```
upkg/
├── upkg.go              # Main entry point & Manager logic
├── cmd/
│   └── upkg/
│       └── main.go      # CLI entry point
├── deps/                # Package registry (name mappings per backend)
│   ├── sqlite3/
│   │   └── index.toml
│   ├── openssl/
│   │   └── index.toml
│   └── curl/
│       └── index.toml
└── pkg/
    ├── backend/         # Backend implementations
    │   ├── winget.go    # Windows Winget logic
    │   ├── nix.go       # Linux/macOS Nix logic
    │   ├── apt.go       # Ubuntu/Debian logic
    │   ├── brew.go      # Homebrew logic
    │   └── ...          # (apk, dnf, dpkg, pacman, zypper)
    ├── registry/        # Registry lookup and alias resolution
    │   └── registry.go
    ├── env/             # Environment management
    │   ├── environment.go
    │   ├── environment_manager.go
    │   ├── library.go
    │   └── constants.go
    ├── index/           # Package index syncing (Git)
    ├── winget/          # Winget parser & driver
    └── nix/             # Nix parser & driver
```

---

## Contributing

### Adding a package to the registry

1. Create a folder under `deps/` with the package name
2. Add an `index.toml` with the native package name for each backend
3. Submit a Pull Request

Example — adding `zlib`:
```
deps/
└── zlib/
    └── index.toml
```

```toml
# deps/zlib/index.toml

aliases = ["z"]

[backends]
apt     = "zlib1g-dev"
dpkg    = "zlib1g-dev"
dnf     = "zlib-devel"
brew    = "zlib"
winget  = "zlib.zlib"
pacman  = "zlib"
apk     = "zlib-dev"
zypper  = "zlib-devel"
choco   = "zlib"
nix     = "zlib"
```

### Adding a new backend

1. Fork the repo
2. Create a feature branch
3. Implement the `Backend` interface in `pkg/backend/`
4. Submit a Pull Request

---

## License

Licensed under the MIT License.