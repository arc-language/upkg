// cmd/upkg/main.go
package main

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "strings"
    
    "github.com/arc-language/upkg"
    "github.com/arc-language/upkg/pkg/backend"
    "github.com/arc-language/upkg/pkg/env"
)

var envManager *env.EnvironmentManager

func main() {
    envManager = env.NewEnvironmentManager("")
    
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }
    
    command := os.Args[1]
    args := os.Args[2:]
    
    switch command {
    case "init":
        handleInitCommand(args)
    case "env":
        handleEnvCommand(args)
    case "install":
        handleInstallCommand(args)
    case "run":
        handleRunCommand(args)
    case "shell":
        handleShellCommand(args)
    case "info":
        handleInfoCommand(args)
    case "search":
        handleSearchCommand(args)
    case "list":
        handleListCommand(args)
    case "version", "--version", "-v":
        fmt.Println("upkg version 0.1.0")
    case "help", "--help", "-h":
        printUsage()
    default:
        fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
        printUsage()
        os.Exit(1)
    }
}

func printUsage() {
    fmt.Println(`upkg - Universal Package Manager

Usage: upkg <command> [args]

Setup (run once):
  init [bash|zsh|fish]          Generate shell integration code
                                Add to your shell RC: eval "$(upkg init)"

Environment Management:
  env create <name> [--backend apt|brew|nix|dnf|pacman]
                                Create new isolated environment
  env list                      List all environments
  env activate <name>           Activate an environment (modifies shell)
  env deactivate                Deactivate current environment
  env remove <name>             Remove an environment
  env info [name]               Show environment details

Package Management:
  install <package>             Install package to active environment
  search <query>                Search for packages
  list                          List installed packages in active environment
  info <package>                Show package information
  run <command> [args...]       Run command in active environment

Options:
  --help, -h                    Show this help message
  --version, -v                 Show version

Setup Instructions:
  1. Add to your ~/.bashrc or ~/.zshrc:
     eval "$(upkg init)"
  
  2. Reload your shell:
     source ~/.bashrc

  3. Create and activate an environment:
     upkg env create myproject
     upkg env activate myproject
  
  4. Your prompt will now show: (myproject) user@host:~$

Examples:
  # One-time setup
  echo 'eval "$(upkg init)"' >> ~/.bashrc
  source ~/.bashrc
  
  # Create and use environment
  upkg env create myproject --backend apt
  upkg env activate myproject    # Shell automatically modified!
  
  # Install packages
  upkg install gcc wget
  
  # Now commands just work
  gcc --version
  wget https://example.com
  
  # Deactivate
  upkg env deactivate
`)
}

func handleInitCommand(args []string) {
    shell := "bash" // default
    
    if len(args) > 0 {
        shell = args[0]
    } else {
        // Auto-detect from SHELL env var
        if shellEnv := os.Getenv("SHELL"); shellEnv != "" {
            parts := strings.Split(shellEnv, "/")
            shell = parts[len(parts)-1]
        }
    }
    
    switch shell {
    case "bash":
        printBashInit()
    case "zsh":
        printZshInit()
    case "fish":
        printFishInit()
    default:
        fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
        fmt.Fprintf(os.Stderr, "Supported: bash, zsh, fish\n")
        os.Exit(1)
    }
}

func printBashInit() {
    fmt.Print(`# upkg shell integration
# Add this to your ~/.bashrc or ~/.bash_profile:
# eval "$(upkg init bash)"

upkg() {
    if [[ "$1" == "env" ]] && [[ "$2" == "activate" ]]; then
        # Activation needs to modify current shell
        if [[ -z "$3" ]]; then
            echo "Error: upkg env activate requires an environment name" >&2
            return 1
        fi
        
        # Call upkg to activate and get shell code
        command upkg env activate "$3" > /dev/null 2>&1
        
        if [[ $? -ne 0 ]]; then
            command upkg env activate "$3"
            return 1
        fi
        
        # Source the environment
        eval "$(command upkg shell)"
        
        # Set environment variable for prompt
        export UPKG_ENV="$3"
        
    elif [[ "$1" == "env" ]] && [[ "$2" == "deactivate" ]]; then
        # Deactivation
        command upkg env deactivate
        unset UPKG_ENV
        
        # Clear the environment by reloading shell defaults
        if [[ -n "$UPKG_OLD_PATH" ]]; then
            export PATH="$UPKG_OLD_PATH"
            unset UPKG_OLD_PATH
        fi
        
    else
        # For all other commands, just call the binary
        command upkg "$@"
    fi
}

# Modify prompt to show active environment
__upkg_prompt_command() {
    if [[ -n "$UPKG_ENV" ]]; then
        # Remove old upkg prompt if exists
        PS1="${PS1#\(*\) }"
        # Add new upkg prompt
        PS1="($UPKG_ENV) $PS1"
    else
        # Remove upkg prompt
        PS1="${PS1#\(*\) }"
    fi
}

# Add to PROMPT_COMMAND
if [[ -z "$PROMPT_COMMAND" ]]; then
    PROMPT_COMMAND="__upkg_prompt_command"
else
    PROMPT_COMMAND="__upkg_prompt_command;$PROMPT_COMMAND"
fi
`)
}

func printZshInit() {
    fmt.Print(`# upkg shell integration for Zsh
# Add this to your ~/.zshrc:
# eval "$(upkg init zsh)"

upkg() {
    if [[ "$1" == "env" ]] && [[ "$2" == "activate" ]]; then
        # Activation needs to modify current shell
        if [[ -z "$3" ]]; then
            echo "Error: upkg env activate requires an environment name" >&2
            return 1
        fi
        
        # Call upkg to activate and get shell code
        command upkg env activate "$3" > /dev/null 2>&1
        
        if [[ $? -ne 0 ]]; then
            command upkg env activate "$3"
            return 1
        fi
        
        # Source the environment
        eval "$(command upkg shell)"
        
        # Set environment variable for prompt
        export UPKG_ENV="$3"
        
    elif [[ "$1" == "env" ]] && [[ "$2" == "deactivate" ]]; then
        # Deactivation
        command upkg env deactivate
        unset UPKG_ENV
        
    else
        # For all other commands, just call the binary
        command upkg "$@"
    fi
}

# Modify prompt to show active environment
precmd() {
    if [[ -n "$UPKG_ENV" ]]; then
        PROMPT="%F{green}($UPKG_ENV)%f $PROMPT"
    fi
}
`)
}

func printFishInit() {
    fmt.Print(`# upkg shell integration for Fish
# Add this to your ~/.config/fish/config.fish:
# upkg init fish | source

function upkg
    if test "$argv[1]" = "env" -a "$argv[2]" = "activate"
        if test -z "$argv[3]"
            echo "Error: upkg env activate requires an environment name" >&2
            return 1
        end
        
        # Call upkg to activate
        command upkg env activate $argv[3] > /dev/null 2>&1
        
        if test $status -ne 0
            command upkg env activate $argv[3]
            return 1
        end
        
        # Source the environment
        command upkg shell | source
        
        # Set environment variable
        set -gx UPKG_ENV $argv[3]
        
    else if test "$argv[1]" = "env" -a "$argv[2]" = "deactivate"
        command upkg env deactivate
        set -e UPKG_ENV
        
    else
        command upkg $argv
    end
end

# Update prompt
function fish_prompt
    if set -q UPKG_ENV
        echo -n (set_color green)"($UPKG_ENV)"(set_color normal)" "
    end
    # Your existing prompt here
end
`)
}

func handleEnvCommand(args []string) {
    if len(args) == 0 {
        fmt.Fprintf(os.Stderr, "Error: env command requires a subcommand\n\n")
        fmt.Println("Available subcommands:")
        fmt.Println("  create <name>     Create new environment")
        fmt.Println("  list              List all environments")
        fmt.Println("  activate <name>   Activate environment")
        fmt.Println("  deactivate        Deactivate environment")
        fmt.Println("  remove <name>     Remove environment")
        fmt.Println("  info [name]       Show environment details")
        os.Exit(1)
    }
    
    subcommand := args[0]
    subArgs := args[1:]
    
    switch subcommand {
    case "create":
        handleEnvCreate(subArgs)
    case "list":
        handleEnvList(subArgs)
    case "activate":
        handleEnvActivate(subArgs)
    case "deactivate":
        handleEnvDeactivate(subArgs)
    case "remove", "rm":
        handleEnvRemove(subArgs)
    case "info":
        handleEnvInfo(subArgs)
    default:
        fmt.Fprintf(os.Stderr, "Unknown env subcommand: %s\n", subcommand)
        os.Exit(1)
    }
}

func handleEnvCreate(args []string) {
    if len(args) < 1 {
        fmt.Fprintf(os.Stderr, "Usage: upkg env create <name> [--backend apt|brew|nix|dnf|pacman]\n")
        os.Exit(1)
    }
    
    name := args[0]
    backend := "apt" // default
    
    // Parse --backend flag
    for i := 1; i < len(args); i++ {
        if args[i] == "--backend" && i+1 < len(args) {
            backend = args[i+1]
            i++
        }
    }
    
    // Validate backend
    validBackends := map[string]bool{
        "apt": true, "brew": true, "nix": true,
        "dnf": true, "pacman": true, "apk": true,
        "zypper": true, "choco": true,
    }
    
    if !validBackends[backend] {
        fmt.Fprintf(os.Stderr, "Error: invalid backend '%s'\n", backend)
        fmt.Fprintf(os.Stderr, "Valid backends: apt, brew, nix, dnf, pacman, apk, zypper, choco\n")
        os.Exit(1)
    }
    
    envSpec, err := envManager.CreateEnv(name, backend)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("✓ Created environment: %s\n", envSpec.Name)
    fmt.Printf("  Backend: %s\n", envSpec.Backend)
    fmt.Printf("  Path: %s\n", envSpec.InstallPath)
    fmt.Printf("\nNext steps:\n")
    fmt.Printf("  upkg env activate %s\n", name)
    fmt.Printf("  upkg install <package>\n")
}

func handleEnvList(args []string) {
    envs, err := envManager.ListEnvs()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    if len(envs) == 0 {
        fmt.Println("No environments found.")
        fmt.Println("\nCreate one with: upkg env create <name>")
        return
    }
    
    active, _ := envManager.GetActiveEnv()
    
    fmt.Println("Environments:")
    for _, e := range envs {
        marker := "  "
        if active != nil && e.Name == active.Name {
            marker = "* "
        }
        fmt.Printf("%s%s\n", marker, e.Name)
        fmt.Printf("     Backend: %s | Packages: %d\n", e.Backend, len(e.Packages))
    }
    
    if active != nil {
        fmt.Printf("\n* = active environment\n")
    }
}

func handleEnvActivate(args []string) {
    if len(args) < 1 {
        fmt.Fprintf(os.Stderr, "Usage: upkg env activate <name>\n")
        os.Exit(1)
    }
    
    name := args[0]
    if err := envManager.ActivateEnv(name); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("✓ Environment '%s' activated.\n", name)
}

func handleEnvDeactivate(args []string) {
    if err := envManager.DeactivateEnv(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("✓ Environment deactivated")
}

func handleEnvRemove(args []string) {
    if len(args) < 1 {
        fmt.Fprintf(os.Stderr, "Usage: upkg env remove <name>\n")
        os.Exit(1)
    }
    
    name := args[0]
    
    // Check if it's the active environment
    if active, err := envManager.GetActiveEnv(); err == nil && active.Name == name {
        fmt.Fprintf(os.Stderr, "Error: cannot remove active environment '%s'\n", name)
        fmt.Fprintf(os.Stderr, "Deactivate it first: upkg env deactivate\n")
        os.Exit(1)
    }
    
    // Confirm deletion
    fmt.Printf("Remove environment '%s'? [y/N]: ", name)
    var response string
    fmt.Scanln(&response)
    
    if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
        fmt.Println("Cancelled")
        return
    }
    
    if err := envManager.RemoveEnv(name); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("✓ Environment '%s' removed\n", name)
}

func handleEnvInfo(args []string) {
    var envSpec *env.EnvSpec
    var err error
    
    if len(args) > 0 {
        // Show info for specific environment
        envSpec, err = envManager.LoadEnv(args[0])
    } else {
        // Show info for active environment
        envSpec, err = envManager.GetActiveEnv()
    }
    
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    active, _ := envManager.GetActiveEnv()
    isActive := active != nil && active.Name == envSpec.Name
    
    fmt.Printf("Environment: %s", envSpec.Name)
    if isActive {
        fmt.Printf(" (active)")
    }
    fmt.Println()
    fmt.Printf("  Backend: %s\n", envSpec.Backend)
    fmt.Printf("  Path: %s\n", envSpec.InstallPath)
    fmt.Printf("  Created: %s\n", envSpec.CreatedAt)
    fmt.Printf("  Packages: %d\n", len(envSpec.Packages))
    
    if len(envSpec.Packages) > 0 {
        fmt.Println("\nInstalled packages:")
        for name, version := range envSpec.Packages {
            fmt.Printf("  - %s (%s)\n", name, version)
        }
    }
    
    // Show environment paths
    environment := envSpec.GetEnvironment()
    
    if libs := environment.GetLibraryPaths(); len(libs) > 0 {
        fmt.Println("\nLibrary paths:")
        for _, p := range libs {
            fmt.Printf("  - %s\n", p)
        }
    }
    
    if bins := environment.GetBinaryPaths(); len(bins) > 0 {
        fmt.Println("\nBinary paths:")
        for _, p := range bins {
            fmt.Printf("  - %s\n", p)
        }
    }
}

func handleInstallCommand(args []string) {
    if len(args) == 0 {
        fmt.Fprintf(os.Stderr, "Usage: upkg install <package> [package...]\n")
        os.Exit(1)
    }
    
    envSpec, err := envManager.GetActiveEnv()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: No active environment\n\n")
        fmt.Fprintf(os.Stderr, "Create and activate an environment first:\n")
        fmt.Fprintf(os.Stderr, "  upkg env create myenv\n")
        fmt.Fprintf(os.Stderr, "  upkg env activate myenv\n")
        os.Exit(1)
    }
    
    // Create upkg manager with environment's install path
    config := upkg.DefaultConfig()
    config.InstallPath = envSpec.InstallPath
    config.Debug = false // Set to true for verbose output
    
    backendType := mapBackendName(envSpec.Backend)
    manager, err := upkg.NewManager(backendType, config)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating manager: %v\n", err)
        os.Exit(1)
    }
    defer manager.Close()
    
    // Install each package
    for _, packageName := range args {
        fmt.Printf("Installing %s...\n", packageName)
        
        pkg := &backend.Package{Name: packageName}
        if err := manager.Download(context.Background(), pkg, nil); err != nil {
            fmt.Fprintf(os.Stderr, "✗ Error installing %s: %v\n", packageName, err)
            continue
        }
        
        // Record installation
        envSpec.AddPackage(packageName, "latest")
        
        fmt.Printf("✓ %s installed successfully\n", packageName)
    }
    
    // Save updated environment
    envManager.UpdateEnv(envSpec)
}

func handleRunCommand(args []string) {
    if len(args) == 0 {
        fmt.Fprintf(os.Stderr, "Usage: upkg run <command> [args...]\n")
        os.Exit(1)
    }
    
    envSpec, err := envManager.GetActiveEnv()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: No active environment\n")
        os.Exit(1)
    }
    
    environment := envSpec.GetEnvironment()
    
    // Execute command with environment
    cmd := exec.Command(args[0], args[1:]...)
    cmd.Env = environment.BuildEnv()
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    
    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            os.Exit(exitErr.ExitCode())
        }
        os.Exit(1)
    }
}

func handleShellCommand(args []string) {
    envSpec, err := envManager.GetActiveEnv()
    if err != nil {
        // No active environment - output nothing (clean for eval)
        return
    }
    
    environment := envSpec.GetEnvironment()
    
    // Generate shell code for eval (NOT a .sh file, just stdout)
    shellCode := environment.GenerateActivateScript()
    fmt.Print(shellCode)
}

func handleInfoCommand(args []string) {
    if len(args) == 0 {
        fmt.Fprintf(os.Stderr, "Usage: upkg info <package>\n")
        os.Exit(1)
    }
    
    envSpec, err := envManager.GetActiveEnv()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: No active environment\n")
        os.Exit(1)
    }
    
    packageName := args[0]
    
    // Create manager
    config := upkg.DefaultConfig()
    config.InstallPath = envSpec.InstallPath
    
    backendType := mapBackendName(envSpec.Backend)
    manager, err := upkg.NewManager(backendType, config)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    defer manager.Close()
    
    // Get package info
    info, err := manager.GetInfo(context.Background(), packageName)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("Package: %s\n", info.Name)
    fmt.Printf("Version: %s\n", info.Version)
    if info.Description != "" {
        fmt.Printf("Description: %s\n", info.Description)
    }
    if info.Homepage != "" {
        fmt.Printf("Homepage: %s\n", info.Homepage)
    }
    if info.License != "" {
        fmt.Printf("License: %s\n", info.License)
    }
    if len(info.Platforms) > 0 {
        fmt.Printf("Platforms: %s\n", strings.Join(info.Platforms, ", "))
    }
}

func handleSearchCommand(args []string) {
    if len(args) == 0 {
        fmt.Fprintf(os.Stderr, "Usage: upkg search <query>\n")
        os.Exit(1)
    }
    
    envSpec, err := envManager.GetActiveEnv()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: No active environment\n")
        os.Exit(1)
    }
    
    query := strings.Join(args, " ")
    
    // Create manager
    config := upkg.DefaultConfig()
    config.InstallPath = envSpec.InstallPath
    
    backendType := mapBackendName(envSpec.Backend)
    manager, err := upkg.NewManager(backendType, config)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    defer manager.Close()
    
    // Search packages
    fmt.Printf("Searching for '%s'...\n\n", query)
    results, err := manager.Search(context.Background(), query)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    if len(results) == 0 {
        fmt.Println("No packages found")
        return
    }
    
    fmt.Printf("Found %d packages:\n\n", len(results))
    for i, pkg := range results {
        if i >= 20 {
            fmt.Printf("... and %d more\n", len(results)-20)
            break
        }
        
        fmt.Printf("%s (%s)\n", pkg.Name, pkg.Version)
        if pkg.Description != "" {
            desc := pkg.Description
            if len(desc) > 80 {
                desc = desc[:77] + "..."
            }
            fmt.Printf("  %s\n", desc)
        }
        fmt.Println()
    }
}

func handleListCommand(args []string) {
    envSpec, err := envManager.GetActiveEnv()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: No active environment\n")
        os.Exit(1)
    }
    
    if len(envSpec.Packages) == 0 {
        fmt.Println("No packages installed in this environment")
        return
    }
    
    fmt.Printf("Packages in environment '%s':\n\n", envSpec.Name)
    for name, version := range envSpec.Packages {
        fmt.Printf("  %s (%s)\n", name, version)
    }
    fmt.Printf("\nTotal: %d packages\n", len(envSpec.Packages))
}

func mapBackendName(name string) backend.BackendType {
    switch strings.ToLower(name) {
    case "apt":
        return backend.BackendApt
    case "brew":
        return backend.BackendBrew
    case "nix":
        return backend.BackendNix
    case "dnf":
        return backend.BackendDnf
    case "pacman":
        return backend.BackendPacman
    case "apk":
        return backend.BackendApk
    case "zypper":
        return backend.BackendZypper
    case "choco":
        return backend.BackendChoco
    default:
        return backend.BackendAuto
    }
}