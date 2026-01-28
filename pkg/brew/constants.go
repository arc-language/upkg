// constants.go
package brew

const (
	// DefaultAPIURL is the Homebrew formula API endpoint
	DefaultAPIURL = "https://formulae.brew.sh/api"

	// DefaultRegistryURL is the GitHub Container Registry for Homebrew
	DefaultRegistryURL = "https://ghcr.io/v2/homebrew/core"

	// DefaultInstallPathIntel is the default Homebrew install path for Intel Macs
	DefaultInstallPathIntel = "/usr/local"

	// DefaultInstallPathARM is the default Homebrew install path for ARM Macs
	DefaultInstallPathARM = "/opt/homebrew"

	// DefaultCellar is the Cellar subdirectory name
	DefaultCellar = "Cellar"
)