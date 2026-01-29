package zypper

const (
	// DefaultMirror is the main OpenSUSE download server
	DefaultMirror = "http://download.opensuse.org"

	// DefaultDistribution uses Tumbleweed (Rolling) for maximum package availability
	DefaultDistribution = "tumbleweed"
	
	// DefaultInstallPath is where packages will be extracted
	DefaultInstallPath = "/opt/upkg"

	// DefaultArch is the primary architecture
	DefaultArch = "x86_64"
)

// Repository paths relative to distribution
const (
	RepoOSS      = "repo/oss"      // Open Source Software (Main)
	RepoNonOSS   = "repo/non-oss"  // Proprietary Software
	RepoUpdate   = "repo/update"   // Updates
)

// DefaultRepos lists the standard repositories enabled by default
var DefaultRepos = []string{
	RepoOSS,
}