// pkg/core/package.go
package core

// Package represents a universal package across all backends
type Package struct {
	Name        string   // Package name
	Version     string   // Package version
	Description string   // Package description
	Backend     string   // Which backend manages this package
	Platform    string   // Target platform
	Installed   bool     // Whether the package is installed
}