// pkg/env/library.go
package env

import (
    "os"
    "path/filepath"
    "strings"
)

// FindLibrary searches for a specific library by name
// Returns the first match found in library search paths
func (e *Environment) FindLibrary(name string) *Library {
    searchPaths := e.GetLibraryPaths()
    extensions := GetLibraryExtensions()
    
    for _, dir := range searchPaths {
        for _, ext := range extensions {
            // Try lib{name}{ext} pattern (e.g., libssl.so)
            filename := "lib" + name + ext
            fullPath := filepath.Join(dir, filename)
            
            if fileExists(fullPath) {
                return &Library{
                    Name:     name,
                    Path:     fullPath,
                    Type:     ext,
                    IsStatic: ext == ".a" || ext == ".lib",
                }
            }
            
            // Try versioned: lib{name}{ext}.* (e.g., libssl.so.3)
            matches, _ := filepath.Glob(filepath.Join(dir, filename+"*"))
            if len(matches) > 0 {
                // Return first match
                return &Library{
                    Name:     name,
                    Path:     matches[0],
                    Type:     ext,
                    IsStatic: ext == ".a" || ext == ".lib",
                }
            }
        }
    }
    
    return nil
}

// FindSharedLibrary searches specifically for shared libraries (.so, .dylib, .dll)
func (e *Environment) FindSharedLibrary(name string) *Library {
    searchPaths := e.GetLibraryPaths()
    extensions := GetSharedLibraryExtensions()
    
    for _, dir := range searchPaths {
        for _, ext := range extensions {
            filename := "lib" + name + ext
            fullPath := filepath.Join(dir, filename)
            
            if fileExists(fullPath) {
                return &Library{
                    Name:     name,
                    Path:     fullPath,
                    Type:     ext,
                    IsStatic: false,
                }
            }
            
            matches, _ := filepath.Glob(filepath.Join(dir, filename+"*"))
            if len(matches) > 0 {
                return &Library{
                    Name:     name,
                    Path:     matches[0],
                    Type:     ext,
                    IsStatic: false,
                }
            }
        }
    }
    
    return nil
}

// FindStaticLibrary searches specifically for static libraries (.a, .lib)
func (e *Environment) FindStaticLibrary(name string) *Library {
    searchPaths := e.GetLibraryPaths()
    extensions := GetStaticLibraryExtensions()
    
    for _, dir := range searchPaths {
        for _, ext := range extensions {
            filename := "lib" + name + ext
            fullPath := filepath.Join(dir, filename)
            
            if fileExists(fullPath) {
                return &Library{
                    Name:     name,
                    Path:     fullPath,
                    Type:     ext,
                    IsStatic: true,
                }
            }
        }
    }
    
    return nil
}

// FindAllLibraries returns all libraries in the environment
func (e *Environment) FindAllLibraries() []*Library {
    var libraries []*Library
    searchPaths := e.GetLibraryPaths()
    extensions := GetLibraryExtensions()
    
    seen := make(map[string]bool) // Avoid duplicates
    
    for _, dir := range searchPaths {
        entries, err := os.ReadDir(dir)
        if err != nil {
            continue
        }
        
        for _, entry := range entries {
            if entry.IsDir() {
                continue
            }
            
            name := entry.Name()
            
            // Check if file has library extension
            for _, ext := range extensions {
                if strings.HasSuffix(name, ext) || strings.Contains(name, ext+".") {
                    fullPath := filepath.Join(dir, name)
                    
                    if seen[fullPath] {
                        continue
                    }
                    seen[fullPath] = true
                    
                    // Extract library name (remove "lib" prefix and extension)
                    libName := name
                    if strings.HasPrefix(libName, "lib") {
                        libName = libName[3:]
                    }
                    // Remove extension and version
                    libName = strings.Split(libName, ".")[0]
                    
                    libraries = append(libraries, &Library{
                        Name:     libName,
                        Path:     fullPath,
                        Type:     ext,
                        IsStatic: ext == ".a" || ext == ".lib",
                    })
                    break
                }
            }
        }
    }
    
    return libraries
}

// FindAllSharedLibraries returns all shared libraries
func (e *Environment) FindAllSharedLibraries() []*Library {
    all := e.FindAllLibraries()
    var shared []*Library
    
    for _, lib := range all {
        if !lib.IsStatic {
            shared = append(shared, lib)
        }
    }
    
    return shared
}

// FindAllStaticLibraries returns all static libraries
func (e *Environment) FindAllStaticLibraries() []*Library {
    all := e.FindAllLibraries()
    var static []*Library
    
    for _, lib := range all {
        if lib.IsStatic {
            static = append(static, lib)
        }
    }
    
    return static
}

// HasLibrary checks if a library exists in the environment
func (e *Environment) HasLibrary(name string) bool {
    return e.FindLibrary(name) != nil
}

// ListLibraryNames returns names of all libraries found
func (e *Environment) ListLibraryNames() []string {
    libs := e.FindAllLibraries()
    names := make([]string, 0, len(libs))
    seen := make(map[string]bool)
    
    for _, lib := range libs {
        if !seen[lib.Name] {
            names = append(names, lib.Name)
            seen[lib.Name] = true
        }
    }
    
    return names
}