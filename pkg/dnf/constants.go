// pkg/dnf/constants.go
package dnf

const (
	// DefaultRepositoryURL is the main Fedora repository
	DefaultRepositoryURL = "https://dl.fedoraproject.org/pub/fedora/linux"

	// DefaultRelease is the default Fedora release
	DefaultRelease = "39" // Fedora 39 (latest stable as of now)

	// DefaultRepository is the default repository name
	DefaultRepository = "releases"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"
)

// Common Fedora releases
const (
	ReleaseFedora39     = "39" // Latest stable
	ReleaseFedora38     = "38" // Previous stable
	ReleaseFedora37     = "37" // Old stable
	ReleaseRawhide      = "rawhide" // Bleeding edge
)

// Fedora repositories
const (
	RepoReleases   = "releases" // Stable releases
	RepoUpdates    = "updates"  // Updates to stable
)