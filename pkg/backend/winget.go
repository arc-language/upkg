// pkg/backend/winget.go
package backend

import (
	"context"

	"github.com/arc-language/upkg/pkg/winget"
)

type WingetBackend struct {
	manager *winget.PackageManager
	config  *Config
}

func NewWingetBackend(config *Config) (*WingetBackend, error) {
	if config == nil {
		config = DefaultConfig()
	}

	wConfig := &winget.Config{
		InstallPath: config.InstallPath,
		CachePath:   config.CachePath,
		Timeout:     config.Timeout,
		Debug:       config.Debug,
		Logger:      config.Logger,
	}

	return &WingetBackend{
		manager: winget.NewPackageManager(wConfig),
		config:  config,
	}, nil
}

func (b *WingetBackend) Download(ctx context.Context, pkg *Package, opts *DownloadOptions) error {
	wOpts := &winget.DownloadOptions{
		Package:      pkg.Name,
		Version:      pkg.Version,
		Extract:      derefBool(opts.Extract, true),
		KeepArchive:  derefBool(opts.KeepArchive, false),
		VerifyHash:   derefBool(opts.VerifyHash, true),
	}
	
	if opts.Platform != "" {
		wOpts.Architecture = opts.Platform
	}

	return b.manager.Download(ctx, wOpts)
}

func (b *WingetBackend) GetInfo(ctx context.Context, name string) (*PackageInfo, error) {
	m, err := b.manager.GetInfo(ctx, name)
	if err != nil {
		return nil, err
	}

	return &PackageInfo{
		Name:        m.PackageName,
		Version:     m.PackageVersion,
		Description: m.ShortDescription,
		Homepage:    "", // Not always in root of manifest
		License:     m.License,
		Backend:     "winget",
	}, nil
}

func (b *WingetBackend) Search(ctx context.Context, query string) ([]*PackageInfo, error) {
	results, err := b.manager.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var infos []*PackageInfo
	for _, r := range results {
		infos = append(infos, &PackageInfo{
			Name:        r.Latest.Name, // Or r.ID
			Version:     "",            // Latest version implied
			Description: r.Latest.Description,
			Homepage:    r.Latest.Homepage,
			Backend:     "winget",
		})
	}
	return infos, nil
}

func (b *WingetBackend) Name() string {
	return "winget"
}

func (b *WingetBackend) Close() error {
	return nil
}