// pkg/choco/parser.go
package choco

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// AtomFeed represents the NuGet V2 API response
type AtomFeed struct {
	XMLName xml.Name     `xml:"feed"`
	Entries []AtomEntry  `xml:"entry"`
}

// AtomEntry represents a package entry in the feed
type AtomEntry struct {
	ID      string         `xml:"id"`
	Title   string         `xml:"title"`
	Summary string         `xml:"summary"`
	Updated string         `xml:"updated"`
	Author  AtomAuthor     `xml:"author"`
	Content AtomContent    `xml:"content"`
	Props   PackageProps   `xml:"properties"`
}

// AtomAuthor represents the author element
type AtomAuthor struct {
	Name string `xml:"name"`
}

// AtomContent represents the content element with package URL
type AtomContent struct {
	Type string `xml:"type,attr"`
	Src  string `xml:"src,attr"`
}

// PackageProps represents the metadata properties
type PackageProps struct {
	ID              string `xml:"Id"`
	Version         string `xml:"Version"`
	Title           string `xml:"Title"`
	Description     string `xml:"Description"`
	Summary         string `xml:"Summary"`
	Authors         string `xml:"Authors"`
	Owners          string `xml:"Owners"`
	ProjectURL      string `xml:"ProjectUrl"`
	LicenseURL      string `xml:"LicenseUrl"`
	IconURL         string `xml:"IconUrl"`
	Tags            string `xml:"Tags"`
	Dependencies    string `xml:"Dependencies"`
	PackageHash     string `xml:"PackageHash"`
	PackageHashAlgo string `xml:"PackageHashAlgorithm"`
	PackageSize     string `xml:"PackageSize"`
	Published       string `xml:"Published"`
	DownloadCount   string `xml:"DownloadCount"`
}

// ParseFeed parses the NuGet V2 API Atom feed response
func ParseFeed(r io.Reader) ([]*PackageInfo, error) {
	var feed AtomFeed
	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&feed); err != nil {
		return nil, fmt.Errorf("decoding feed: %w", err)
	}

	var packages []*PackageInfo
	for _, entry := range feed.Entries {
		pkg := &PackageInfo{
			ID:              entry.Props.ID,
			Version:         entry.Props.Version,
			Title:           entry.Title,
			Description:     strings.TrimSpace(entry.Props.Description),
			Summary:         strings.TrimSpace(entry.Props.Summary),
			Authors:         entry.Props.Authors,
			Owners:          entry.Props.Owners,
			ProjectURL:      entry.Props.ProjectURL,
			LicenseURL:      entry.Props.LicenseURL,
			IconURL:         entry.Props.IconURL,
			Tags:            entry.Props.Tags,
			PackageHash:     entry.Props.PackageHash,
			PackageHashAlgo: entry.Props.PackageHashAlgo,
			Published:       entry.Props.Published,
		}

		// Parse dependencies
		if entry.Props.Dependencies != "" {
			pkg.Dependencies = parseDependencies(entry.Props.Dependencies)
		}

		// Parse package size
		if entry.Props.PackageSize != "" {
			if size, err := strconv.ParseInt(entry.Props.PackageSize, 10, 64); err == nil {
				pkg.PackageSize = size
			}
		}

		// Parse download count
		if entry.Props.DownloadCount != "" {
			if count, err := strconv.ParseInt(entry.Props.DownloadCount, 10, 64); err == nil {
				pkg.DownloadCount = count
			}
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// parseDependencies parses the dependency string
func parseDependencies(deps string) []string {
	if deps == "" {
		return nil
	}

	var result []string
	parts := strings.Split(deps, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			// Format: "id:version:targetFramework"
			// We just want the ID
			if idx := strings.Index(part, ":"); idx > 0 {
				result = append(result, part[:idx])
			} else {
				result = append(result, part)
			}
		}
	}
	return result
}