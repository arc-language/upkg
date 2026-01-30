// pkg/env/env-manager.go
package env

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// EnvSpec represents an isolated package environment (like conda env)
type EnvSpec struct {
    Name        string            `json:"name"`
    InstallPath string            `json:"install_path"`
    Backend     string            `json:"backend"`
    Packages    map[string]string `json:"packages"` // name -> version
    CreatedAt   string            `json:"created_at"`
}

// EnvironmentManager manages conda-style environments
type EnvironmentManager struct {
    rootDir string // ~/.upkg/envs
}

// NewEnvironmentManager creates environment manager
func NewEnvironmentManager(rootDir string) *EnvironmentManager {
    if rootDir == "" {
        home, _ := os.UserHomeDir()
        rootDir = filepath.Join(home, ".upkg", "envs")
    }
    
    os.MkdirAll(rootDir, 0755)
    
    return &EnvironmentManager{
        rootDir: rootDir,
    }
}

// CreateEnv creates a new isolated environment
func (em *EnvironmentManager) CreateEnv(name, backend string) (*EnvSpec, error) {
    envPath := filepath.Join(em.rootDir, name)
    
    if _, err := os.Stat(envPath); err == nil {
        return nil, fmt.Errorf("environment '%s' already exists", name)
    }
    
    // Create environment directory
    if err := os.MkdirAll(envPath, 0755); err != nil {
        return nil, fmt.Errorf("creating environment directory: %w", err)
    }
    
    env := &EnvSpec{
        Name:        name,
        InstallPath: envPath,
        Backend:     backend,
        Packages:    make(map[string]string),
        CreatedAt:   time.Now().Format(time.RFC3339),
    }
    
    // Save environment metadata
    if err := em.saveEnv(env); err != nil {
        return nil, err
    }
    
    return env, nil
}

// ListEnvs lists all environments
func (em *EnvironmentManager) ListEnvs() ([]*EnvSpec, error) {
    entries, err := os.ReadDir(em.rootDir)
    if err != nil {
        if os.IsNotExist(err) {
            return []*EnvSpec{}, nil
        }
        return nil, err
    }
    
    var envs []*EnvSpec
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        env, err := em.LoadEnv(entry.Name())
        if err != nil {
            continue
        }
        envs = append(envs, env)
    }
    
    return envs, nil
}

// LoadEnv loads an environment by name
func (em *EnvironmentManager) LoadEnv(name string) (*EnvSpec, error) {
    metaPath := filepath.Join(em.rootDir, name, "env.json")
    
    data, err := os.ReadFile(metaPath)
    if err != nil {
        return nil, fmt.Errorf("environment '%s' not found", name)
    }
    
    var env EnvSpec
    if err := json.Unmarshal(data, &env); err != nil {
        return nil, err
    }
    
    return &env, nil
}

// RemoveEnv removes an environment
func (em *EnvironmentManager) RemoveEnv(name string) error {
    envPath := filepath.Join(em.rootDir, name)
    return os.RemoveAll(envPath)
}

// ActivateEnv marks an environment as active
func (em *EnvironmentManager) ActivateEnv(name string) error {
    // Verify environment exists
    if _, err := em.LoadEnv(name); err != nil {
        return err
    }
    
    // Write active environment to state file
    statePath := filepath.Join(em.rootDir, ".active")
    return os.WriteFile(statePath, []byte(name), 0644)
}

// GetActiveEnv returns the currently active environment
func (em *EnvironmentManager) GetActiveEnv() (*EnvSpec, error) {
    statePath := filepath.Join(em.rootDir, ".active")
    
    data, err := os.ReadFile(statePath)
    if err != nil {
        return nil, fmt.Errorf("no active environment")
    }
    
    name := string(data)
    return em.LoadEnv(name)
}

// DeactivateEnv clears the active environment
func (em *EnvironmentManager) DeactivateEnv() error {
    statePath := filepath.Join(em.rootDir, ".active")
    return os.Remove(statePath)
}

// GetEnvironment returns Environment for an env spec
func (e *EnvSpec) GetEnvironment() *Environment {
    return New(e.InstallPath, e.Backend)
}

// AddPackage records a package installation
func (e *EnvSpec) AddPackage(name, version string) {
    e.Packages[name] = version
}

// saveEnv saves environment metadata
func (em *EnvironmentManager) saveEnv(env *EnvSpec) error {
    metaPath := filepath.Join(env.InstallPath, "env.json")
    
    data, err := json.MarshalIndent(env, "", "  ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(metaPath, data, 0644)
}

// UpdateEnv saves changes to environment
func (em *EnvironmentManager) UpdateEnv(env *EnvSpec) error {
    return em.saveEnv(env)
}