// pkg/apk/constants.go
package apk

const (
	// DefaultRepositoryURL is the main Alpine repository (CDN)
	DefaultRepositoryURL = "https://dl-cdn.alpinelinux.org/alpine"

	// AlternativeRepositoryURL is an alternative mirror
	AlternativeRepositoryURL = "https://mirrors.alpinelinux.org/alpine"

	// DefaultBranch is the default Alpine branch to use
	DefaultBranch = "v3.19" // Alpine 3.19 (latest stable)

	// DefaultRepository is the default repository name
	DefaultRepository = "main"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"
)

// Common Alpine branches
const (
	BranchEdge  = "edge"    // Bleeding edge
	BranchV3_19 = "v3.19"   // Latest stable
	BranchV3_18 = "v3.18"   // Previous stable
	BranchV3_17 = "v3.17"   // Old stable
	BranchV3_16 = "v3.16"   // Old stable
	BranchV3_15 = "v3.15"   // Old stable
)

// Alpine repositories
const (
	RepoMain      = "main"      // Main packages
	RepoCommunity = "community" // Community packages
	RepoTesting   = "testing"   // Testing packages (edge only)
)