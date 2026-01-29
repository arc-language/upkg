// pkg/choco/platform.go
package choco

import (
	"fmt"
	"runtime"
)

// DetectPlatform checks if we're on Windows
func DetectPlatform() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("chocolatey backend only supports Windows, got: %s", runtime.GOOS)
	}
	return nil
}