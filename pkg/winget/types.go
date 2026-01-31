// pkg/winget/types.go
package winget

// APIResponse represents a generic paginated response from winget.run
type APIResponse[T any] struct {
	Data       []T    `json:"Data"`
	Total      int    `json:"Total"`
	Page       int    `json:"Page"`
	TotalPages int    `json:"TotalPages"`
}

// PackageEntry represents a package summary from the search API
type PackageEntry struct {
	ID          string      `json:"Id"`
	Versions    []string    `json:"Versions"`
	Latest      VersionInfo `json:"Latest"`
	UpdatedAt   string      `json:"UpdatedAt"` // Changed to string to handle non-standard API time formats
	SearchScore float64     `json:"SearchScore,omitempty"`
}

// VersionInfo contains metadata about a specific version
type VersionInfo struct {
	Name        string   `json:"Name"`
	Publisher   string   `json:"Publisher"`
	Description string   `json:"Description"`
	Homepage    string   `json:"Homepage"`
	License     string   `json:"License"`
	LicenseUrl  string   `json:"LicenseUrl"`
	Tags        []string `json:"Tags"`
}

// Manifest represents the full package manifest (usually a singleton or merged view)
// Note: This matches the structure returned by /v2/manifests/{id}/{version}
type Manifest struct {
	PackageIdentifier string      `json:"PackageIdentifier"`
	PackageVersion    string      `json:"PackageVersion"`
	PackageLocale     string      `json:"PackageLocale"`
	Publisher         string      `json:"Publisher"`
	PackageName       string      `json:"PackageName"`
	License           string      `json:"License"`
	ShortDescription  string      `json:"ShortDescription"`
	Installers        []Installer `json:"Installers"`
}

// Installer represents a single installation option
type Installer struct {
	Architecture    string            `json:"Architecture"` // x64, x86, arm64
	InstallerUrl    string            `json:"InstallerUrl"`
	InstallerSha256 string            `json:"InstallerSha256"`
	InstallerType   string            `json:"InstallerType"` // exe, msi, msix, zip, etc.
	Scope           string            `json:"Scope"`         // user, machine
	InstallModes    []string          `json:"InstallModes"`  // silent, interactive
	Switches        map[string]string `json:"Switches"`      // Silent, SilentWithProgress
}

// DownloadOptions configures the download behavior
type DownloadOptions struct {
	Package      string
	Version      string
	Architecture string // Go runtime format (amd64), will be converted
	Extract      bool
	KeepArchive  bool
	VerifyHash   bool
}