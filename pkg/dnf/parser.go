// pkg/dnf/parser.go
package dnf

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseRepoMD parses a repomd.xml file
func ParseRepoMD(r io.Reader) (*RepoMD, error) {
	type XMLRepoMD struct {
		XMLName  xml.Name `xml:"repomd"`
		Revision string   `xml:"revision"`
		Data     []struct {
			Type     string `xml:"type,attr"`
			Location struct {
				Href string `xml:"href,attr"`
			} `xml:"location"`
			Checksum struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"checksum"`
			OpenChecksum struct {
				Type  string `xml:"type,attr"`
				Value string `xml:",chardata"`
			} `xml:"open-checksum"`
			Timestamp int64 `xml:"timestamp"`
			Size      int64 `xml:"size"`
			OpenSize  int64 `xml:"open-size"`
		} `xml:"data"`
	}

	var xmlRepo XMLRepoMD
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&xmlRepo); err != nil {
		return nil, fmt.Errorf("decoding repomd.xml: %w", err)
	}

	repoMD := &RepoMD{
		Revision: xmlRepo.Revision,
	}

	for _, d := range xmlRepo.Data {
		repoMD.Data = append(repoMD.Data, RepoData{
			Type:         d.Type,
			Location:     d.Location.Href,
			Checksum:     d.Checksum.Value,
			OpenChecksum: d.OpenChecksum.Value,
			Timestamp:    d.Timestamp,
			Size:         d.Size,
			OpenSize:     d.OpenSize,
		})
	}

	return repoMD, nil
}

// ParsePrimary parses a primary.xml file (package metadata)
func ParsePrimary(r io.Reader) ([]*PackageInfo, error) {
	type XMLPackage struct {
		Name    string `xml:"name"`
		Arch    string `xml:"arch"`
		Version struct {
			Epoch   string `xml:"epoch,attr"`
			Ver     string `xml:"ver,attr"`
			Rel     string `xml:"rel,attr"`
		} `xml:"version"`
		Summary     string `xml:"summary"`
		Description string `xml:"description"`
		URL         string `xml:"url"`
		Packager    string `xml:"packager"`
		Size        struct {
			Package   int64 `xml:"package,attr"`
			Installed int64 `xml:"installed,attr"`
		} `xml:"size"`
		Location struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
		Checksum struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"checksum"`
		Format struct {
			License  string `xml:"http://linux.duke.edu/metadata/rpm license"`
			Vendor   string `xml:"http://linux.duke.edu/metadata/rpm vendor"`
			Provides struct {
				Entries []struct {
					Name string `xml:"name,attr"`
				} `xml:"http://linux.duke.edu/metadata/rpm entry"`
			} `xml:"http://linux.duke.edu/metadata/rpm provides"`
			Requires struct {
				Entries []struct {
					Name string `xml:"name,attr"`
				} `xml:"http://linux.duke.edu/metadata/rpm entry"`
			} `xml:"http://linux.duke.edu/metadata/rpm requires"`
			Conflicts struct {
				Entries []struct {
					Name string `xml:"name,attr"`
				} `xml:"http://linux.duke.edu/metadata/rpm entry"`
			} `xml:"http://linux.duke.edu/metadata/rpm conflicts"`
			Obsoletes struct {
				Entries []struct {
					Name string `xml:"name,attr"`
				} `xml:"http://linux.duke.edu/metadata/rpm entry"`
			} `xml:"http://linux.duke.edu/metadata/rpm obsoletes"`
		} `xml:"format"`
	}

	type XMLMetadata struct {
		XMLName  xml.Name     `xml:"metadata"`
		Packages []XMLPackage `xml:"package"`
	}

	var metadata XMLMetadata
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decoding primary.xml: %w", err)
	}

	var packages []*PackageInfo
	for _, p := range metadata.Packages {
		pkg := &PackageInfo{
			Name:          p.Name,
			Version:       p.Version.Ver,
			Release:       p.Version.Rel,
			Epoch:         p.Version.Epoch,
			Architecture:  p.Arch,
			Summary:       strings.TrimSpace(p.Summary),
			Description:   strings.TrimSpace(p.Description),
			URL:           p.URL,
			License:       p.Format.License,
			Vendor:        p.Format.Vendor,
			Packager:      p.Packager,
			Size:          p.Size.Package,
			InstalledSize: p.Size.Installed,
			Location:      p.Location.Href,
			Checksum:      p.Checksum.Value,
			ChecksumType:  p.Checksum.Type,
		}

		// Parse provides
		for _, entry := range p.Format.Provides.Entries {
			if entry.Name != "" {
				pkg.Provides = append(pkg.Provides, entry.Name)
			}
		}

		// Parse requires
		for _, entry := range p.Format.Requires.Entries {
			if entry.Name != "" && !strings.HasPrefix(entry.Name, "rpmlib(") {
				pkg.Requires = append(pkg.Requires, entry.Name)
			}
		}

		// Parse conflicts
		for _, entry := range p.Format.Conflicts.Entries {
			if entry.Name != "" {
				pkg.Conflicts = append(pkg.Conflicts, entry.Name)
			}
		}

		// Parse obsoletes
		for _, entry := range p.Format.Obsoletes.Entries {
			if entry.Name != "" {
				pkg.Obsoletes = append(pkg.Obsoletes, entry.Name)
			}
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// FullVersion returns the full version string (epoch:version-release)
func (p *PackageInfo) FullVersion() string {
	if p.Epoch != "" && p.Epoch != "0" {
		return fmt.Sprintf("%s:%s-%s", p.Epoch, p.Version, p.Release)
	}
	return fmt.Sprintf("%s-%s", p.Version, p.Release)
}

// NVRA returns the Name-Version-Release.Architecture format
func (p *PackageInfo) NVRA() string {
	return fmt.Sprintf("%s-%s.%s", p.Name, p.FullVersion(), p.Architecture)
}