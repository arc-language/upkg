package pacman

const (
	// DefaultMirror is a reliable, global Tier 1 mirror
	// In a real scenario, this should be configurable or parsed from /etc/pacman.d/mirrorlist
	DefaultMirror = "https://geo.mirror.pkgbuild.com"

	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"

	// DefaultArch is the primary architecture for Arch Linux
	DefaultArch = "x86_64"
)

// Repository names
const (
	RepoCore     = "core"     // Critical system packages
	RepoExtra    = "extra"    // General application packages
	RepoMultilib = "multilib" // 32-bit compatibility libraries
)

// DefaultRepos lists the standard repositories enabled by default
var DefaultRepos = []string{
	RepoCore,
	RepoExtra,
}