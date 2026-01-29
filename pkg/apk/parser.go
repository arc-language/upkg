// pkg/apk/parser.go
package apk

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseAPKINDEX parses an Alpine APKINDEX file
func ParseAPKINDEX(r io.Reader) ([]*PackageInfo, error) {
	// APKINDEX.tar.gz contains a single file named "APKINDEX"
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Read the first (and only) file from the tar
	_, err = tarReader.Next()
	if err != nil {
		return nil, fmt.Errorf("reading tar entry: %w", err)
	}

	return parseAPKINDEXContent(tarReader)
}

// parseAPKINDEXContent parses the content of an APKINDEX file
func parseAPKINDEXContent(r io.Reader) ([]*PackageInfo, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var packages []*PackageInfo
	var current *PackageInfo

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line indicates end of package stanza
		if line == "" {
			if current != nil {
				packages = append(packages, current)
				current = nil
			}
			continue
		}

		// Each line is in format "X:value" where X is a single character field code
		if len(line) < 2 || line[1] != ':' {
			continue
		}

		field := line[0]
		value := strings.TrimSpace(line[2:])

		// Start new package if we see P: field
		if field == 'P' {
			if current != nil {
				packages = append(packages, current)
			}
			current = &PackageInfo{
				Package: value,
			}
			continue
		}

		if current == nil {
			continue
		}

		// Parse fields based on Alpine APKINDEX format
		switch field {
		case 'V': // Version
			current.Version = value
		case 'A': // Architecture
			current.Architecture = value
		case 'S': // Package size
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				current.PackageSize = size
			}
		case 'I': // Installed size
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				current.InstalledSize = size
			}
		case 'T': // Description
			current.Description = value
		case 'U': // URL
			current.URL = value
		case 'L': // License
			current.License = value
		case 'o': // Origin
			current.Origin = value
		case 'm': // Maintainer
			current.Maintainer = value
		case 't': // Build time
			if t, err := strconv.ParseInt(value, 10, 64); err == nil {
				current.BuildTime = t
			}
		case 'c': // Commit
			current.Commit = value
		case 'D': // Dependencies
			current.Depends = parseAPKList(value)
		case 'p': // Provides
			current.Provides = parseAPKList(value)
		case 'i': // Install if
			current.InstallIf = parseAPKList(value)
		case 'C': // Checksum (SHA256)
			current.Checksum = value
		}
	}

	// Don't forget the last package
	if current != nil {
		packages = append(packages, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning APKINDEX: %w", err)
	}

	return packages, nil
}

// parseAPKList parses a space-separated list of packages
func parseAPKList(s string) []string {
	if s == "" {
		return nil
	}
	
	var result []string
	parts := strings.Fields(s)
	for _, part := range parts {
		// Remove version constraints like >=1.0
		if idx := strings.IndexAny(part, ">=<~"); idx > 0 {
			part = part[:idx]
		}
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}