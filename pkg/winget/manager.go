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
		client: NewClient(cfg.Timeout, logger),
		config: cfg,
		logger: logger,
	}
}

func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	pm.logger.Printf("Searching for package: %s", opts.Package)

	// 1. Resolve Package ID and Version
	id := opts.Package
	version := opts.Version
	
	if version == "" || version == "latest" {
		results, err := pm.client.Search(ctx, opts.Package)
		if err != nil {
			return fmt.Errorf("searching for package: %w", err)
		}
		if len(results) == 0 {
			return fmt.Errorf("package '%s' not found", opts.Package)
		}
		
		// Attempt to find exact match or fallback to first
		var entry PackageEntry
		found := false
		for _, p := range results {
			if strings.EqualFold(p.ID, opts.Package) || strings.EqualFold(p.Latest.Name, opts.Package) {
				entry = p
				found = true
				break
			}
		}
		if !found {
			entry = results[0]
		}
		
		id = entry.ID
		if len(entry.Versions) > 0 {
			// Try the last version in the list (usually latest)
			version = entry.Versions[len(entry.Versions)-1]
		} else {
			version = "latest"
		}
		pm.logger.Printf("Resolved: %s @ %s", id, version)
	}

	// 2. Get Manifest (with fallback)
	manifest, err := pm.client.GetManifest(ctx, id, version)
	if err != nil {
		// If specific version failed, try "latest" literal as fallback
		pm.logger.Printf("Failed to get version %s, trying 'latest'...", version)
		manifest, err = pm.client.GetManifest(ctx, id, "latest")
		if err != nil {
			return fmt.Errorf("fetching manifest: %w", err)
		}
		version = "latest"
	}

	// 3. Select Installer
	targetArch := opts.Architecture
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	wingetArch, ok := SupportedArchitectures[targetArch]
	if !ok {
		wingetArch = targetArch 
	}

	var bestInstaller *Installer
	for _, inst := range manifest.Installers {
		if strings.EqualFold(inst.Architecture, wingetArch) {
			bestInstaller = &inst
			break
		}
		// Fallback: x64 can run x86
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
	// Sanitize filename
	fileName = strings.ReplaceAll(fileName, "/", "_")
	fileName = strings.ReplaceAll(fileName, "\\", "_")
	
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
	installDir := filepath.Join(pm.config.InstallPath, manifest.PackageIdentifier)
	
	if opts.Extract && (bestInstaller.InstallerType == InstallerTypeZip || strings.HasSuffix(bestInstaller.InstallerUrl, ".zip")) {
		pm.logger.Printf("Extracting zip to %s", installDir)
		if err := unzip(destPath, installDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		pm.logger.Printf("Moving installer to %s", installDir)
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return err
		}
		finalPath := filepath.Join(installDir, fileName)
		
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
	results, err := pm.client.Search(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("package not found")
	}
	
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