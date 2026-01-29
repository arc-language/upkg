// pkg/apt/constants.go
package apt

const (
	// DefaultRepositoryURL is the main Ubuntu repository
	DefaultRepositoryURL = "http://archive.ubuntu.com/ubuntu"

	// DefaultSecurityURL is the Ubuntu security repository
	DefaultSecurityURL = "http://security.ubuntu.com/ubuntu"

	// DefaultPortsURL is for ARM and other architectures
	DefaultPortsURL = "http://ports.ubuntu.com/ubuntu-ports"

	// DefaultRelease is the default Ubuntu release to use
	DefaultRelease = "noble" // Ubuntu 24.04 LTS

	// DefaultComponent is the default repository component
	DefaultComponent = "main"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"
)

// Common Ubuntu releases
const (
	ReleaseNoble   = "noble"   // 24.04 LTS (April 2024)
	ReleaseMantic  = "mantic"  // 23.10
	ReleaseLunar   = "lunar"   // 23.04
	ReleaseJammy   = "jammy"   // 22.04 LTS
	ReleaseImpish  = "impish"  // 21.10
	ReleaseHirsute = "hirsute" // 21.04
	ReleaseFocal   = "focal"   // 20.04 LTS
	ReleaseBionic  = "bionic"  // 18.04 LTS
	ReleaseXenial  = "xenial"  // 16.04 LTS (EOL)
)

// Repository components
const (
	ComponentMain       = "main"       // Officially supported open-source software
	ComponentUniverse   = "universe"   // Community-maintained open-source software
	ComponentRestricted = "restricted" // Proprietary drivers
	ComponentMultiverse = "multiverse" // Software restricted by copyright or legal issues
)