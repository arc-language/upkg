// pkg/zypper/parser.go
package zypper

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// ParseRepomd finds the location of the 'primary' metadata file from repomd.xml
func ParseRepomd(r io.Reader) (string, error) {
	var repo Repomd
	if err := xml.NewDecoder(r).Decode(&repo); err != nil {
		return "", err
	}

	for _, data := range repo.Data {
		if data.Type == "primary" {
			return data.Location.Href, nil
		}
	}

	return "", fmt.Errorf("primary metadata not found in repomd.xml")
}

// ParsePrimary parses the primary package metadata
// filename is used to determine compression type (gz or zst)
func ParsePrimary(r io.Reader, filename string, repoName string) ([]*PackageInfo, error) {
	var xmlReader io.Reader
	var err error

	// Handle compression based on file extension
	if strings.HasSuffix(filename, ".zst") {
		decoder, err := zstd.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer decoder.Close()
		xmlReader = decoder
	} else if strings.HasSuffix(filename, ".gz") {
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gzReader.Close()
		xmlReader = gzReader
	} else {
		// Assume uncompressed
		xmlReader = r
	}

	// XML Stream
	decoder := xml.NewDecoder(xmlReader)
	var packages []*PackageInfo

	for {
		t, _ := decoder.Token()
		if t == nil {
			break
		}

		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "package" {
				var p PrimaryPackage
				if err := decoder.DecodeElement(&p, &se); err != nil {
					continue
				}

				// Only process rpms
				if p.Type != "rpm" {
					continue
				}

				// Construct Version String (Ver-Rel)
				fullVersion := p.Version.Ver
				if p.Version.Rel != "" {
					fullVersion += "-" + p.Version.Rel
				}

				info := &PackageInfo{
					Name:          p.Name,
					Version:       fullVersion,
					Architecture:  p.Arch,
					Summary:       p.Summary,
					Description:   p.Description,
					URL:           p.Url,
					License:       p.Format.License,
					Packager:      p.Packager,
					Size:          p.Size.Package,
					InstalledSize: p.Size.Installed,
					Location:      p.Location.Href,
					Checksum:      p.Checksum.Value,
					ChecksumType:  p.Checksum.Type,
					Repository:    repoName,
				}
				packages = append(packages, info)
			}
		}
	}

	return packages, nil
}