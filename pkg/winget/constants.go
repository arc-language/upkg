// pkg/winget/constants.go
package winget

const (
	// APIBaseURL is the base URL for the winget.run community API
	APIBaseURL = "https://api.winget.run/v2"

	// Installer Types
	InstallerTypeExe      = "exe"
	InstallerTypeMsi      = "msi"
	InstallerTypeMsix     = "msix"
	InstallerTypeZip      = "zip"
	InstallerTypeInno     = "inno"
	InstallerTypeNullsoft = "nullsoft"
	InstallerTypeWix      = "wix"
	InstallerTypeBurn     = "burn"
	InstallerTypePortable = "portable"
)

// SupportedArchitectures maps Go architectures to Winget architectures
var SupportedArchitectures = map[string]string{
	"amd64": "x64",
	"386":   "x86",
	"arm64": "arm64",
	"arm":   "arm",
}