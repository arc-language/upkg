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
# Create an isolated environment
upkg env create myproject --backend apt

# Activate it
upkg env activate myproject
eval "$(upkg shell)"

# Install packages
upkg install gcc wget openssl

# Now commands just work!
gcc --version
wget https://example.com
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
cd cmd/upkg
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

### Environment Management

Create isolated package environments for different projects:
```bash
# Create a new environment
upkg env create myproject --backend apt

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
# 1. Create environment for a C++ project
upkg env create cpp-dev --backend apt
upkg env activate cpp-dev
eval "$(upkg shell)"

# 2. Install development tools
upkg install gcc g++ cmake make

# 3. Work on your project
gcc myprogram.c -o myprogram
./myprogram

# 4. Create another environment for Python
upkg env create python-ml --backend brew
upkg env activate python-ml
eval "$(upkg shell)"

# 5. Install Python tools
upkg install python3 numpy

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
    
    // 2. Create a new environment
    envSpec, err := envMgr.CreateEnv("myproject", "apt")
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. Activate it
    envMgr.ActivateEnv("myproject")
    
    // 4. Install packages to this environment
    config := upkg.DefaultConfig()
    config.InstallPath = envSpec.InstallPath
    
    mgr, err := upkg.NewManager(upkg.BackendApt, config)
    if err != nil {
        log.Fatal(err)
    }
    defer mgr.Close()
    
    pkg := &upkg.Package{Name: "gcc"}
    if err := mgr.Download(context.Background(), pkg, nil); err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Successfully installed gcc!")
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
├── cmd/
│   └── upkg/
│       └── main.go      # CLI entry point
└── pkg/
    ├── backend/         # Backend implementations
    │   ├── apt.go       # Ubuntu/Debian logic
    │   ├── brew.go      # Homebrew logic
    │   ├── choco.go     # Windows Chocolatey logic
    │   ├── nix.go       # Nix logic
    │   └── ...          # (apk, dnf, dpkg, pacman, zypper)
    ├── env/             # Environment management
    │   ├── environment.go
    │   ├── environment_manager.go
    │   ├── library.go
    │   └── constants.go
    ├── apt/             # APT package manager implementation
    └── ...              # Internal package manager drivers
```

---

## Features

- **Cross-platform**: Works on Linux, macOS, and Windows
- **Backend flexibility**: Choose from apt, dnf, brew, nix, choco, and more
- **environments**: Keep project dependencies separate
- **Smart library detection**: Automatically finds libraries and headers
- **Compiler integration**: Generate flags for gcc, clang, etc.
- **No system pollution**: Install packages without affecting system

---

## Contributing

1. Fork the repo
2. Create a feature branch
3. Implement the `Backend` interface in `pkg/backend/`
4. Submit a Pull Request

---

## License

Licensed under the MIT License.