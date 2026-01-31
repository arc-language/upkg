package winget

// PackageEntry represents a package summary from the search API
type PackageEntry struct {
	ID          string        `json:"Id"`
	RawVersions []interface{} `json:"Versions"` // Handle []string or []object
	Latest      VersionInfo   `json:"Latest"`
	UpdatedAt   string        `json:"UpdatedAt"`
}

// GetVersions safely extracts version strings
func (p *PackageEntry) GetVersions() []string {
	var versions []string
	if p.RawVersions == nil {
		return versions
	}
	for _, v := range p.RawVersions {
		if s, ok := v.(string); ok {
			versions = append(versions, s)
		} else if m, ok := v.(map[string]interface{}); ok {
			if ver, ok := m["Version"].(string); ok {
				versions = append(versions, ver)
			}
		}
	}
	return versions
}

// VersionInfo contains metadata about a specific version
type VersionInfo struct {
	Version     string   `json:"Version"` // Often missing in API v2
	Name        string   `json:"Name"`
	Publisher   string   `json:"Publisher"`
}

// Manifest represents the full package manifest
type Manifest struct {
	PackageIdentifier string      `json:"PackageIdentifier"`
	PackageVersion    string      `json:"PackageVersion"`
	PackageName       string      `json:"PackageName"`
	Installers        []Installer `json:"Installers"`
}

// Installer represents a single installation option
type Installer struct {
	Architecture    string            `json:"Architecture"`
	InstallerUrl    string            `json:"InstallerUrl"`
	InstallerSha256 string            `json:"InstallerSha256"`
	InstallerType   string            `json:"InstallerType"`
}

type DownloadOptions struct {
	Package      string
	Version      string
	Architecture string
	Extract      bool
	KeepArchive  bool
	VerifyHash   bool
}