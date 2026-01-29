// pkg/dpkg/parser.go
package dpkg

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ParsePackages parses a Debian Packages file
func ParsePackages(r io.Reader) ([]*PackageInfo, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // Handle large descriptions

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

		// Continuation line (starts with space or tab)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if current != nil && current.Description != "" {
				current.Description += "\n" + strings.TrimSpace(line)
			}
			continue
		}

		// Parse field: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Start new package if we see Package field
		if field == "Package" {
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

		// Parse fields
		switch field {
		case "Version":
			current.Version = value
		case "Architecture":
			current.Architecture = value
		case "Maintainer":
			current.Maintainer = value
		case "Installed-Size":
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				current.InstalledSize = size * 1024 // Convert from KB to bytes
			}
		case "Depends":
			current.Depends = parsePackageList(value)
		case "Recommends":
			current.Recommends = parsePackageList(value)
		case "Suggests":
			current.Suggests = parsePackageList(value)
		case "Conflicts":
			current.Conflicts = parsePackageList(value)
		case "Replaces":
			current.Replaces = parsePackageList(value)
		case "Provides":
			current.Provides = parsePackageList(value)
		case "Description":
			current.Description = value
		case "Homepage":
			current.Homepage = value
		case "Section":
			current.Section = value
		case "Priority":
			current.Priority = value
		case "Filename":
			current.Filename = value
		case "Size":
			if size, err := strconv.ParseInt(value, 10, 64); err == nil {
				current.Size = size
			}
		case "MD5sum":
			current.MD5sum = value
		case "SHA1":
			current.SHA1 = value
		case "SHA256":
			current.SHA256 = value
		case "SHA512":
			current.SHA512 = value
		}
	}

	// Don't forget the last package
	if current != nil {
		packages = append(packages, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning packages file: %w", err)
	}

	return packages, nil
}

// parsePackageList parses a comma-separated package dependency list
func parsePackageList(s string) []string {
	var result []string
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Remove version constraints like (>= 1.0)
		if idx := strings.Index(part, "("); idx != -1 {
			part = strings.TrimSpace(part[:idx])
		}
		// Remove alternative dependencies (|)
		if idx := strings.Index(part, "|"); idx != -1 {
			part = strings.TrimSpace(part[:idx])
		}
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// ParseRelease parses a Debian Release file
func ParseRelease(r io.Reader) (*Release, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	release := &Release{}
	var currentHashType string

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Hash sections start with the hash type and colon
		if !strings.HasPrefix(line, " ") {
			currentHashType = ""
		}

		// Continuation line (hash entry)
		if strings.HasPrefix(line, " ") {
			if currentHashType != "" {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					size, _ := strconv.ParseInt(parts[1], 10, 64)
					fileHash := FileHash{
						Hash: parts[0],
						Size: size,
						Name: parts[2],
					}

					switch currentHashType {
					case "MD5Sum":
						release.MD5Sum = append(release.MD5Sum, fileHash)
					case "SHA1":
						release.SHA1 = append(release.SHA1, fileHash)
					case "SHA256":
						release.SHA256 = append(release.SHA256, fileHash)
					case "SHA512":
						release.SHA512 = append(release.SHA512, fileHash)
					}
				}
			}
			continue
		}

		// Parse field: value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch field {
		case "Origin":
			release.Origin = value
		case "Label":
			release.Label = value
		case "Suite":
			release.Suite = value
		case "Version":
			release.Version = value
		case "Codename":
			release.Codename = value
		case "Date":
			if t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", value); err == nil {
				release.Date = t
			}
		case "Architectures":
			release.Architectures = strings.Fields(value)
		case "Components":
			release.Components = strings.Fields(value)
		case "Description":
			release.Description = value
		case "MD5Sum":
			currentHashType = "MD5Sum"
		case "SHA1":
			currentHashType = "SHA1"
		case "SHA256":
			currentHashType = "SHA256"
		case "SHA512":
			currentHashType = "SHA512"
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning release file: %w", err)
	}

	return release, nil
}