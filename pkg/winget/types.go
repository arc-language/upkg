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
	ID          string        `json:"Id"`
	RawVersions []interface{} `json:"Versions"` // Handle both []string and []object
	Latest      VersionInfo   `json:"Latest"`
	UpdatedAt   string        `json:"UpdatedAt"`
	SearchScore float64       `json:"SearchScore,omitempty"`
}

// GetVersions extracts version strings from the raw versions field
func (p *PackageEntry) GetVersions() []string {
	var versions []string
	if p.RawVersions == nil {
		return versions
	}
	for _, v := range p.RawVersions {
		if s, ok := v.(string); ok {
			versions = append(versions, s)
		} else if m, ok := v.(map[string]interface{}); ok {
			// Handle case where versions are objects
			if ver, ok := m["Version"].(string); ok {
				versions = append(versions, ver)
			}
		}
	}
	return versions
}

// VersionInfo contains metadata about a specific version
type VersionInfo struct {
	Version     string   `json:"Version"`
	Name        string   `json:"Name"`
	Publisher   string   `json:"Publisher"`
	Description string   `json:"Description"` // Restored
	Homepage    string   `json:"Homepage"`    // Restored
	License     string   `json:"License"`
	LicenseUrl  string   `json:"LicenseUrl"`
	Tags        []string `json:"Tags"`
}

// Manifest represents the full package manifest
type Manifest struct {
	PackageIdentifier string      `json:"PackageIdentifier"`
	PackageVersion    string      `json:"PackageVersion"`
	PackageLocale     string      `json:"PackageLocale"`
	Publisher         string      `json:"Publisher"`
	PackageName       string      `json:"PackageName"`
	License           string      `json:"License"`          // Restored
	ShortDescription  string      `json:"ShortDescription"` // Restored
	Installers        []Installer `json:"Installers"`
}

// Installer represents a single installation option
type Installer struct {
	Architecture    string            `json:"Architecture"`
	InstallerUrl    string            `json:"InstallerUrl"`
	InstallerSha256 string            `json:"InstallerSha256"`
	InstallerType   string            `json:"InstallerType"`
	Scope           string            `json:"Scope"`
	InstallModes    []string          `json:"InstallModes"`
	Switches        map[string]string `json:"Switches"`
}

// DownloadOptions configures the download behavior
type DownloadOptions struct {
	Package      string
	Version      string
	Architecture string
	Extract      bool
	KeepArchive  bool
	VerifyHash   bool
}