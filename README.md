# upkg - Universal Package Manager

A unified package manager written in Go that works across multiple package management systems.

## Supported Backends

- **Nix** - NixOS package manager
- **Homebrew** - macOS/Linux package manager

## Installation
```bash
git clone https://github.com/arc-language/upkg
cd upkg
make build
make install
```

## Usage
```bash
# Install a package (auto-detects backend)
upkg install wget

# Install with specific backend
upkg install wget --backend=nix

# Install specific version
upkg install nginx --version=1.24.0

# Get package information
upkg info python3

# List available backends
upkg list

# Show version
upkg version
```

## Configuration

Create `~/.config/upkg/config.yaml`:
```yaml
default_backend: nix
install_path: ~/.upkg
debug: false
```

## Development
```bash
# Build
make build

# Run tests
make test

# Format code
make fmt
```

## License

MIT License