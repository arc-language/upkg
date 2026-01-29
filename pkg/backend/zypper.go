package backend

import (
	"context"
	"fmt"

	"github.com/arc-language/upkg/pkg/zypper"
)

// ZypperBackend implements the Backend interface for OpenSUSE packages
type ZypperBackend struct {
	manager *zypper.PackageManager
	config  *Config
}

// NewZypperBackend creates a new Zypper backend
func NewZypperBackend(config *Config) (*ZypperBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	zypConfig := &zypper.Config{
		MirrorURL:    "http://download.opensuse.org",
		Distribution: "tumbleweed", // Default to rolling
		Repos:        []string{"repo/oss"},
		InstallPath:  config.InstallPath,
		CachePath:    config.CachePath,
		Timeout:      config.Timeout,
		Debug:        config.Debug,
		Logger:       config.Logger,
	}

	manager := zypper.NewPackageManager(zypConfig)

	return &ZypperBackend{
		manager: manager,
		config:  config,
	}, nil
}

func (b *ZypperBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	zOpts := &zypper.DownloadOptions{
		Package:     pkg.Name,
		Version:     pkg.Version,
		Extract:     derefBool(opts.Extract, true),
		KeepArchive: derefBool(opts.KeepArchive, false),
		VerifyHash:  derefBool(opts.VerifyHash, true),
	}

	if opts.Platform != "" {
		zOpts.Architecture = opts.Platform
	}

	return b.manager.Download(ctx, zOpts)
}

func (b *ZypperBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	pkgInfo, err := b.manager.GetPackageInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting package info: %w", err)
	}

	return &PackageInfo{
		Name:        pkgInfo.Name,
		Version:     pkgInfo.Version,
		Description: pkgInfo.Description,
		Homepage:    pkgInfo.URL,
		License:     pkgInfo.License,
		Platforms:   []string{pkgInfo.Architecture},
		Backend:     "zypper",
	}, nil
}

func (b *ZypperBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	packages, err := b.manager.SearchPackages(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("searching packages: %w", err)
	}

	results := make([]*PackageInfo, 0, len(packages))
	for _, pkg := range packages {
		results = append(results, &PackageInfo{
			Name:        pkg.Name,
			Version:     pkg.Version,
			Description: pkg.Description,
			Homepage:    pkg.URL,
			Platforms:   []string{pkg.Architecture},
			Backend:     "zypper",
		})
	}

	return results, nil
}

func (b *ZypperBackend) Name() string {
	return "zypper"
}

func (b *ZypperBackend) Close() error {
	return nil
}