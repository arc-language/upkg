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
	header, err := tarReader.Next()
	if err != nil {
		return nil, fmt.Errorf("reading tar entry: %w", err)
	}

	// Verify we're reading the APKINDEX file
	if header.Name != "APKINDEX" && header.Name != "./APKINDEX" {
		return nil, fmt.Errorf("unexpected file in tar: %s (expected APKINDEX)", header.Name)
	}

	return parseAPKINDEXContent(tarReader)
}

// parseAPKINDEXContent parses the content of an APKINDEX file
func parseAPKINDEXContent(r io.Reader) ([]*PackageInfo, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var packages []*PackageInfo
	var current *PackageInfo
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Empty line indicates end of package stanza
		if line == "" {
			if current != nil {
				packages = append(packages, current)
				current = nil
			}
			continue
		}

		// Each line is in format "X:value" where X is a single character field code
		if len(line) < 2 {
			continue
		}

		// Handle lines that don't have the colon separator
		if line[1] != ':' {
			// Skip malformed lines
			continue
		}

		field := line[0]
		value := ""
		if len(line) > 2 {
			value = strings.TrimSpace(line[2:])
		}

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

		// Skip if we haven't started a package yet
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
			if value != "" {
				current.Depends = parseAPKList(value)
			}
		case 'p': // Provides
			if value != "" {
				current.Provides = parseAPKList(value)
			}
		case 'i': // Install if
			if value != "" {
				current.InstallIf = parseAPKList(value)
			}
		case 'C': // Checksum (SHA1 with Q1 prefix)
			current.Checksum = value
		}
	}

	// Don't forget the last package
	if current != nil {
		packages = append(packages, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning APKINDEX at line %d: %w", lineNum, err)
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages found in APKINDEX (read %d lines)", lineNum)
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
		// Remove version constraints like >=1.0, <2.0, ~1.0
		cleaned := part
		for i, ch := range part {
			if ch == '>' || ch == '<' || ch == '=' || ch == '~' {
				cleaned = part[:i]
				break
			}
		}
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}