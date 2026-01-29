// pkg/dnf/constants.go
package dnf

const (
	// DefaultRepositoryURL is the main Fedora repository
	DefaultRepositoryURL = "https://dl.fedoraproject.org/pub/fedora/linux"

	// DefaultRelease is the default Fedora release
	DefaultRelease = "40" // Fedora 40 (latest stable)

	// DefaultRepository is the default repository name
	DefaultRepository = "releases"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"
)

// Common Fedora releases
const (
	ReleaseFedora40     = "40" // Latest stable
	ReleaseFedora39     = "39" // Previous stable
	ReleaseFedora38     = "38" // Old stable
	ReleaseRawhide      = "development/rawhide" // Bleeding edge
)

// Fedora repositories
const (
	RepoReleases        = "releases"        // Stable releases
	RepoUpdates         = "updates"         // Updates to stable
	RepoModular         = "modular"         // Modular packages
	RepoEverything      = "everything"      // All packages
	RepoDevelopment     = "development"     // Rawhide/development
)

// Repository paths
const (
	RepoBaseOS         = "Everything"  // Base packages path
	RepoAppStream      = "Modular"     // AppStream/Modular packages
)