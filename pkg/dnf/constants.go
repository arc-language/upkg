// pkg/dnf/constants.go
package dnf

const (
	// DefaultRepositoryURL is the main Fedora repository
	DefaultRepositoryURL = "https://dl.fedoraproject.org/pub/fedora/linux"

	// DefaultRelease is the default Fedora release
	DefaultRelease = "42" // Fedora 42 (stable as of April 2025)

	// DefaultRepository is the default repository name
	DefaultRepository = "releases"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"
)

// Common Fedora releases
const (
	ReleaseFedora43     = "43" // Latest stable (October 2025)
	ReleaseFedora42     = "42" // Current stable (April 2025)
	ReleaseFedora41     = "41" // Previous stable
	ReleaseRawhide      = "rawhide" // Bleeding edge
)

// Fedora repositories
const (
	RepoReleases   = "releases" // Stable releases
	RepoUpdates    = "updates"  // Updates to stable
)