package zypper

import (
	"encoding/xml"
	"log"
	"time"
)

// Config configures the Zypper package manager
type Config struct {
	MirrorURL    string        // Base Mirror URL
	Distribution string        // Distribution (tumbleweed, distribution/leap/15.5)
	Repos        []string      // List of repository paths
	InstallPath  string        // Where to install packages
	CachePath    string        // Where to cache downloaded files
	Timeout      time.Duration // Network timeout
	Debug        bool          // Enable debug logging
	Logger       *log.Logger   // Custom logger
}

// PackageManager handles Zypper package operations
type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
	cache  *PackageCache
}

// PackageInfo contains metadata from the primary.xml
type PackageInfo struct {
	Name          string
	Version       string
	Architecture  string
	Summary       string
	Description   string
	URL           string
	License       string
	Packager      string
	Size          int64  // Package Size
	InstalledSize int64  // Size on Disk
	Location      string // Relative path to .rpm
	Checksum      string // SHA checksum
	ChecksumType  string // sha256, sha1, etc.
	Repository    string // Origin repository
	Dependencies  []Dependency // Package dependencies (added for sub-dep support)
}

// Dependency represents a package dependency
type Dependency struct {
	Name    string
	Version string // Optional version constraint
	Flags   string // EQ, GE, LE, GT, LT
	Epoch   string
	Rel     string
}

// DownloadOptions configures package download
type DownloadOptions struct {
	Package      string
	Version      string
	Architecture string
	Extract      bool
	KeepArchive  bool
	VerifyHash   bool
}

// PackageCache caches package index information
type PackageCache struct {
	packages      map[string]*PackageInfo // key: package_name
	lastUpdate    time.Time
	cacheDuration time.Duration
}

// --- XML Structures ---

// Repomd represents repodata/repomd.xml
type Repomd struct {
	XMLName  xml.Name     `xml:"repomd"`
	Revision string       `xml:"revision"`
	Data     []RepomdData `xml:"data"`
}

type RepomdData struct {
	Type     string         `xml:"type,attr"`
	Location RepomdLocation `xml:"location"`
	Checksum RepomdChecksum `xml:"checksum"`
}

type RepomdLocation struct {
	Href string `xml:"href,attr"`
}

type RepomdChecksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// PrimaryMetadata represents the structure of primary.xml
// Note: We stream parse this, so this struct is for inner elements
type PrimaryPackage struct {
	Type        string      `xml:"type,attr"`
	Name        string      `xml:"name"`
	Arch        string      `xml:"arch"`
	Version     VersionInfo `xml:"version"`
	Checksum    Checksum    `xml:"checksum"`
	Summary     string      `xml:"summary"`
	Description string      `xml:"description"`
	Packager    string      `xml:"packager"`
	Url         string      `xml:"url"`
	Time        TimeInfo    `xml:"time"`
	Size        SizeInfo    `xml:"size"`
	Location    Location    `xml:"location"`
	Format      Format      `xml:"format"`
}

type VersionInfo struct {
	Epoch string `xml:"epoch,attr"`
	Ver   string `xml:"ver,attr"`
	Rel   string `xml:"rel,attr"`
}

type Checksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type TimeInfo struct {
	File  int64 `xml:"file,attr"`
	Build int64 `xml:"build,attr"`
}

type SizeInfo struct {
	Package   int64 `xml:"package,attr"`
	Installed int64 `xml:"installed,attr"`
	Archive   int64 `xml:"archive,attr"`
}

type Location struct {
	Href string `xml:"href,attr"`
}

type Format struct {
	License  string       `xml:"license"`
	Requires RpmRequires  `xml:"requires"`
	Provides RpmProvides  `xml:"provides"`
}

// RpmRequires contains the list of package dependencies
type RpmRequires struct {
	Entries []RpmEntry `xml:"entry"`
}

// RpmProvides contains the list of capabilities this package provides
type RpmProvides struct {
	Entries []RpmEntry `xml:"entry"`
}

// RpmEntry represents a single dependency or provide entry
type RpmEntry struct {
	Name  string `xml:"name,attr"`
	Flags string `xml:"flags,attr,omitempty"` // EQ, GE, LE, GT, LT
	Epoch string `xml:"epoch,attr,omitempty"`
	Ver   string `xml:"ver,attr,omitempty"`
	Rel   string `xml:"rel,attr,omitempty"`
}