// pkg/winget/manager.go
package winget

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config configures the Winget manager
type Config struct {
	InstallPath string
	CachePath   string
	Timeout     time.Duration
	Debug       bool
	Logger      *log.Logger
}

type PackageManager struct {
	client *Client
	config *Config
	logger *log.Logger
}

func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}
	
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &PackageManager{
		client: NewClient(cfg.Timeout),
		config: cfg,
		logger: logger,
	}
}

func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	pm.logger.Printf("Searching for manifest: %s (Version: %s)", opts.Package, opts.Version)

	// 1. Resolve Version if empty
	version := opts.Version
	if version == "" || version == "latest" {
		// Search to find the latest version
		results, err := pm.client.Search(ctx, opts.Package)
		if err != nil {
			return fmt.Errorf("searching for package: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("package '%s' not found", opts.Package)
		}
		
		// Exact match preference or first result
		found := false
		for _, p := range results {
			if strings.EqualFold(p.ID, opts.Package) || strings.EqualFold(p.Latest.Name, opts.Package) {
				version = p.Versions[len(p.Versions)-1] // Assuming sorted, usually last is latest
				// Better: use the one marked latest in metadata if available, but API returns list
				// winget.run usually puts latest version at top of versions list or similar. 
				// Let's assume index 0 is latest for now or check p.Latest
				// Actually, p.Versions is usually just strings. Let's pick the last one or default to "latest"
				if len(p.Versions) > 0 {
					version = p.Versions[len(p.Versions)-1]
				}
				opts.Package = p.ID // Normalize ID
				found = true
				break
			}
		}
		if !found {
			// Default to first result
			opts.Package = results[0].ID
			if len(results[0].Versions) > 0 {
				version = results[0].Versions[len(results[0].Versions)-1]
			}
		}
		pm.logger.Printf("Resolved version: %s", version)
	}

	// 2. Get Manifest
	manifest, err := pm.client.GetManifest(ctx, opts.Package, version)
	if err != nil {
		return fmt.Errorf("fetching manifest: %w", err)
	}

	// 3. Select Installer based on Architecture
	targetArch := opts.Architecture
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	wingetArch, ok := SupportedArchitectures[targetArch]
	if !ok {
		// Fallback to x86/x64 if on arm64/windows via translation, but for now strict
		wingetArch = targetArch 
	}

	var bestInstaller *Installer
	for _, inst := range manifest.Installers {
		if strings.EqualFold(inst.Architecture, wingetArch) {
			bestInstaller = &inst
			break
		}
		// Fallback: if we are on x64, we can run x86
		if wingetArch == "x64" && strings.EqualFold(inst.Architecture, "x86") {
			if bestInstaller == nil {
				bestInstaller = &inst
			}
		}
	}

	if bestInstaller == nil {
		return fmt.Errorf("no compatible installer found for architecture %s", wingetArch)
	}

	pm.logger.Printf("Selected installer: %s (%s)", bestInstaller.InstallerType, bestInstaller.Architecture)

	// 4. Download
	fileName := fmt.Sprintf("%s-%s.%s", manifest.PackageName, version, determineExtension(bestInstaller))
	destPath := filepath.Join(pm.config.CachePath, "downloads", fileName)
	
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	pm.logger.Printf("Downloading from: %s", bestInstaller.InstallerUrl)
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	
	if err := pm.client.DownloadFile(ctx, bestInstaller.InstallerUrl, f); err != nil {
		f.Close()
		return fmt.Errorf("downloading file: %w", err)
	}
	f.Close()

	// 5. Verify Hash
	if opts.VerifyHash && bestInstaller.InstallerSha256 != "" {
		pm.logger.Printf("Verifying hash...")
		if err := verifyHash(destPath, bestInstaller.InstallerSha256); err != nil {
			return err
		}
	}

	// 6. Install / Extract
	// Since upkg focuses on environments, we try to extract "portable" contents.
	// If it's a Zip, we unzip. If it's an EXE/MSI, we just move it to the install path
	// or warn the user that it needs manual installation.
	
	installDir := filepath.Join(pm.config.InstallPath, manifest.PackageIdentifier)
	
	if opts.Extract && (bestInstaller.InstallerType == InstallerTypeZip || strings.HasSuffix(bestInstaller.InstallerUrl, ".zip")) {
		pm.logger.Printf("Extracting zip to %s", installDir)
		if err := unzip(destPath, installDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		// For opaque installers (exe, msi), we copy the installer itself to the bin dir
		// so the user can run it, OR if it's "Portable" type, sometimes it's just the exe.
		pm.logger.Printf("Moving installer to %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return err
		}
		finalPath := filepath.Join(installDir, fileName)
		
		// Move/Copy file
		input, _ := os.ReadFile(destPath)
		os.WriteFile(finalPath, input, 0755)
		
		pm.logger.Printf("✓ Package downloaded. Note: This is an installer (%s).", bestInstaller.InstallerType)
		pm.logger.Printf("  Location: %s", finalPath)
	}

	if !opts.KeepArchive {
		os.Remove(destPath)
	}

	return nil
}

func (pm *PackageManager) Search(ctx context.Context, query string) ([]PackageEntry, error) {
	return pm.client.Search(ctx, query)
}

func (pm *PackageManager) GetInfo(ctx context.Context, name string) (*Manifest, error) {
	// Need to resolve ID first if name is fuzzy
	results, err := pm.client.Search(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("package not found")
	}
	
	// Use the first result's ID and latest version
	id := results[0].ID
	version := "latest"
	if len(results[0].Versions) > 0 {
		version = results[0].Versions[len(results[0].Versions)-1]
	}

	return pm.client.GetManifest(ctx, id, version)
}

// Helpers

func determineExtension(i *Installer) string {
	if strings.Contains(i.InstallerUrl, ".zip") {
		return "zip"
	}
	if strings.Contains(i.InstallerUrl, ".msi") {
		return "msi"
	}
	if strings.Contains(i.InstallerUrl, ".msix") {
		return "msix"
	}
	if strings.Contains(i.InstallerUrl, ".exe") {
		return "exe"
	}
	return "bin"
}

func verifyHash(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		
		// Zip Slip protection
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}