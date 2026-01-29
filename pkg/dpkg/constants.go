// pkg/dpkg/constants.go
package dpkg

const (
	// DefaultRepositoryURL is the main Debian repository
	DefaultRepositoryURL = "http://deb.debian.org/debian"

	// DefaultSecurityURL is the Debian security repository
	DefaultSecurityURL = "http://security.debian.org/debian-security"

	// DefaultRelease is the default Debian release to use
	DefaultRelease = "bookworm" // Debian 12

	// DefaultComponent is the default repository component
	DefaultComponent = "main"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"
)

// Common Debian releases
const (
	ReleaseSid       = "sid"       // unstable
	ReleaseTrixie    = "trixie"    // testing (Debian 13)
	ReleaseBookworm  = "bookworm"  // stable (Debian 12)
	ReleaseBullseye  = "bullseye"  // oldstable (Debian 11)
	ReleaseBuster    = "buster"    // oldoldstable (Debian 10)
)

// Repository components
const (
	ComponentMain         = "main"
	ComponentContrib      = "contrib"
	ComponentNonFree      = "non-free"
	ComponentNonFreeFirm = "non-free-firmware"
)