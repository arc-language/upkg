// errors.go
package upkg

import (
	"errors"
	"fmt"
)

var (
	// ErrPackageNotFound indicates the package was not found
	ErrPackageNotFound = errors.New("package not found")

	// ErrInvalidPackage indicates the package specification is invalid
	ErrInvalidPackage = errors.New("invalid package")

	// ErrBackendNotAvailable indicates the backend is not available
	ErrBackendNotAvailable = errors.New("backend not available")

	// ErrHashMismatch indicates a hash verification failure
	ErrHashMismatch = errors.New("hash mismatch")

	// ErrPlatformNotSupported indicates the platform is not supported
	ErrPlatformNotSupported = errors.New("platform not supported")
)

// Error wraps an error with additional context
type Error struct {
	Op      string // Operation that failed
	Package string // Package name if applicable
	Err     error  // Underlying error
}

func (e *Error) Error() string {
	if e.Package != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.Package, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}