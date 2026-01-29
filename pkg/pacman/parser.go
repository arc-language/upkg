package pacman

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseDatabase parses a Pacman sync database (.db file, which is a tar.gz)
func ParseDatabase(r io.Reader, repoName string) ([]*PackageInfo, error) {
	// 1. Decompress Gzip
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzReader.Close()

	// 2. Read Tar
	tarReader := tar.NewReader(gzReader)
	var packages []*PackageInfo

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar entry: %w", err)
		}

		// We only care about the "desc" file inside each package directory
		// Format: package-version/desc
		if strings.HasSuffix(header.Name, "/desc") {
			pkg, err := parseDescFile(tarReader)
			if err != nil {
				// Log warning but continue?
				continue
			}
			pkg.Repository = repoName
			packages = append(packages, pkg)
		}
	}

	return packages, nil
}

// parseDescFile parses the text content of a 'desc' file
func parseDescFile(r io.Reader) (*PackageInfo, error) {
	scanner := bufio.NewScanner(r)
	pkg := &PackageInfo{}
	var currentHeader string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for %HEADER%
		if strings.HasPrefix(line, "%") && strings.HasSuffix(line, "%") {
			currentHeader = line
			continue
		}

		// Parse data based on current header
		switch currentHeader {
		case "%NAME%":
			pkg.Name = line
		case "%VERSION%":
			pkg.Version = line
		case "%BASE%":
			pkg.Base = line
		case "%DESC%":
			pkg.Description = line
		case "%URL%":
			pkg.URL = line
		case "%ARCH%":
			pkg.Architecture = line
		case "%BUILDDATE%":
			if val, err := strconv.ParseInt(line, 10, 64); err == nil {
				pkg.BuildDate = val
			}
		case "%INSTALLDATE%":
			if val, err := strconv.ParseInt(line, 10, 64); err == nil {
				pkg.InstallDate = val
			}
		case "%PACKAGER%":
			pkg.Packager = line
		case "%SIZE%":
			// Installed Size
			if val, err := strconv.ParseInt(line, 10, 64); err == nil {
				pkg.InstalledSize = val
			}
		case "%CSIZE%":
			// Compressed (Download) Size
			if val, err := strconv.ParseInt(line, 10, 64); err == nil {
				pkg.Size = val
			}
		case "%MD5SUM%":
			pkg.MD5Sum = line
		case "%SHA256SUM%":
			pkg.SHA256Sum = line
		case "%FILENAME%":
			pkg.Filename = line
		case "%LICENSE%":
			pkg.License = append(pkg.License, line)
		case "%GROUPS%":
			pkg.Groups = append(pkg.Groups, line)
		case "%DEPENDS%":
			pkg.Depends = append(pkg.Depends, line)
		case "%OPTDEPENDS%":
			pkg.OptDepends = append(pkg.OptDepends, line)
		case "%MAKEDEPENDS%":
			pkg.MakeDepends = append(pkg.MakeDepends, line)
		case "%CHECKDEPENDS%":
			pkg.CheckDepends = append(pkg.CheckDepends, line)
		case "%CONFLICTS%":
			pkg.Conflicts = append(pkg.Conflicts, line)
		case "%PROVIDES%":
			pkg.Provides = append(pkg.Provides, line)
		case "%REPLACES%":
			pkg.Replaces = append(pkg.Replaces, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return pkg, nil
}