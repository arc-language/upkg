// manager.go
package brew

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"encoding/json"
)

// NewPackageManager creates a new Homebrew package manager
func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.RegistryURL == "" {
		cfg.RegistryURL = DefaultRegistryURL
	}
	if cfg.InstallPath == "" {
		if runtime.GOARCH == "arm64" && runtime.GOOS == "darwin" {
			cfg.InstallPath = DefaultInstallPathARM
		} else {
			cfg.InstallPath = DefaultInstallPathIntel
		}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}

	// Setup logger
	logger := cfg.Logger
	if logger == nil {
		if cfg.Debug {
			logger = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}
	}

	pm := &PackageManager{
		client: NewClientWithTimeout(cfg.Timeout),
		config: cfg,
		logger: logger,
	}

	if cfg.Debug {
		pm.logger.Printf("Initialized Homebrew PackageManager")
		pm.logger.Printf("  APIURL: %s", cfg.APIURL)
		pm.logger.Printf("  RegistryURL: %s", cfg.RegistryURL)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  Timeout: %s", cfg.Timeout)
	}

	return pm
}

// Download downloads and installs a Homebrew package
func (pm *PackageManager) Download(ctx context.Context, opts *DownloadOptions) error {
	if opts == nil || opts.Formula == "" {
		return fmt.Errorf("Formula is required in DownloadOptions")
	}

	pm.logger.Printf("Starting download for formula: %s", opts.Formula)

	// Set defaults
	if opts.Platform == "" {
		detected, err := DetectPlatform()
		if err != nil {
			return fmt.Errorf("detecting platform: %w (please specify Platform explicitly)", err)
		}
		opts.Platform = detected
		pm.logger.Printf("Auto-detected platform: %s", opts.Platform)
	} else {
		if !opts.Platform.IsValid() {
			return fmt.Errorf("invalid platform: %s", opts.Platform)
		}
	}

	if opts.Extract {
		opts.Extract = true // Default to extracting
	}
	if !opts.VerifyHash {
		opts.VerifyHash = true // Default to verifying
	}

	pm.logger.Printf("Download options:")
	pm.logger.Printf("  Formula: %s", opts.Formula)
	pm.logger.Printf("  Version: %s", opts.Version)
	pm.logger.Printf("  Platform: %s", opts.Platform)
	pm.logger.Printf("  Extract: %v", opts.Extract)
	pm.logger.Printf("  KeepArchive: %v", opts.KeepArchive)
	pm.logger.Printf("  VerifyHash: %v", opts.VerifyHash)

	// 1. Get formula info from API
	pm.logger.Printf("Step 1: Fetching formula info from API...")
	formula, err := pm.GetFormulaInfo(ctx, opts.Formula)
	if err != nil {
		return fmt.Errorf("getting formula info: %w", err)
	}

	version := opts.Version
	if version == "" {
		version = formula.Versions.Stable
	}
	pm.logger.Printf("  ✓ Formula info retrieved: %s version %s", formula.Name, version)
	pm.logger.Printf("    Description: %s", formula.Description)

	// 2. Get OCI manifest and find bottle
	pm.logger.Printf("Step 2: Fetching OCI manifest...")
	bottleDigest, bottleURL, sha256Hash, err := pm.getBottleInfo(ctx, opts.Formula, version, opts.Platform)
	if err != nil {
		return fmt.Errorf("getting bottle info: %w", err)
	}
	pm.logger.Printf("  ✓ Bottle found for platform %s", opts.Platform)
	pm.logger.Printf("    Digest: %s", bottleDigest)
	pm.logger.Printf("    SHA256: %s", sha256Hash)

	// 3. Download bottle
	pm.logger.Printf("Step 3: Downloading bottle...")
	bottlePath := filepath.Join(pm.config.InstallPath, "downloads", 
		fmt.Sprintf("%s-%s.%s.bottle.tar.gz", opts.Formula, version, opts.Platform))
	
	if err := pm.downloadBottle(ctx, bottleURL, bottlePath); err != nil {
		return fmt.Errorf("downloading bottle: %w", err)
	}
	pm.logger.Printf("  ✓ Download complete")

	// 4. Verify hash
	if opts.VerifyHash && sha256Hash != "" {
		pm.logger.Printf("Step 4: Verifying SHA256 hash...")
		if err := pm.verifyFileHash(bottlePath, sha256Hash); err != nil {
			return fmt.Errorf("hash verification failed: %w", err)
		}
		pm.logger.Printf("  ✓ Hash verified")
	} else {
		pm.logger.Printf("Step 4: Skipping hash verification")
	}

	// 5. Extract bottle
	if opts.Extract {
		pm.logger.Printf("Step 5: Extracting bottle...")
		cellarPath := filepath.Join(pm.config.InstallPath, DefaultCellar)
		if err := pm.extractBottle(bottlePath, cellarPath); err != nil {
			return fmt.Errorf("extracting bottle: %w", err)
		}
		pm.logger.Printf("  ✓ Extraction complete")

		// 6. Cleanup archive if requested
		if !opts.KeepArchive {
			pm.logger.Printf("Step 6: Removing archive file...")
			if err := os.Remove(bottlePath); err != nil {
				pm.logger.Printf("  ⚠️  Warning: failed to remove archive: %v", err)
			} else {
				pm.logger.Printf("  ✓ Archive removed")
			}
		} else {
			pm.logger.Printf("Step 6: Keeping archive file as requested")
		}
	} else {
		pm.logger.Printf("Step 5: Skipping extraction (Extract=false)")
	}

	return nil
}

// GetFormulaInfo retrieves formula information from the API
func (pm *PackageManager) GetFormulaInfo(ctx context.Context, formula string) (*FormulaInfo, error) {
	url := fmt.Sprintf("%s/formula/%s.json", pm.config.APIURL, formula)
	pm.logger.Printf("Fetching formula info from: %s", url)

	var info FormulaInfo
	if err := pm.client.GetJSON(ctx, url, &info); err != nil {
		pm.logger.Printf("✗ Failed to fetch formula info: %v", err)
		return nil, err
	}

	pm.logger.Printf("✓ Successfully fetched formula info")
	return &info, nil
}

// getBottleInfo retrieves bottle information from OCI registry
func (pm *PackageManager) getBottleInfo(ctx context.Context, formula, version string, platform Platform) (string, string, string, error) {
	// Get OCI manifest
	manifestURL := fmt.Sprintf("%s/%s/manifests/%s", pm.config.RegistryURL, formula, version)
	pm.logger.Printf("Fetching OCI manifest from: %s", manifestURL)

	headers := map[string]string{
		"Accept":        "application/vnd.oci.image.index.v1+json",
		"Authorization": "Bearer QQ==", // Empty bearer token required
	}

	resp, err := pm.client.GetWithHeaders(ctx, manifestURL, headers)
	if err != nil {
		return "", "", "", fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	var manifest OCIManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return "", "", "", fmt.Errorf("decoding manifest: %w", err)
	}

	// Find matching platform
	ociArch := platform.ToOCI()
	for _, m := range manifest.Manifests {
		if m.Platform.Architecture == ociArch {
			// Extract bottle digest from annotations
			bottleDigest, ok := m.Annotations["sh.brew.bottle.digest"]
			if !ok {
				continue
			}

			// Get the actual download URL
			blobURL := fmt.Sprintf("%s/%s/blobs/sha256:%s", pm.config.RegistryURL, formula, bottleDigest)
			
			// We must manually handle the request to prevent following redirects,
			// because the Location header is in the 307 response.
			req, err := http.NewRequestWithContext(ctx, "HEAD", blobURL, nil)
			if err != nil {
				return "", "", "", fmt.Errorf("creating request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer QQ==")
			req.Header.Set("User-Agent", pm.client.userAgent)

			// Create a custom client that does NOT follow redirects
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
				Timeout: pm.config.Timeout,
			}
			
			headResp, err := client.Do(req)
			if err != nil {
				return "", "", "", fmt.Errorf("getting blob location: %w", err)
			}
			defer headResp.Body.Close()

			var downloadURL string
			if headResp.StatusCode == http.StatusOK {
				// If 200, the blob is served directly from the registry
				downloadURL = blobURL
			} else if headResp.StatusCode == http.StatusTemporaryRedirect || headResp.StatusCode == 307 || headResp.StatusCode == http.StatusFound {
				// If 307, the blob is served via redirect (e.g. to objects.githubusercontent.com)
				downloadURL = headResp.Header.Get("Location")
				if downloadURL == "" {
					return "", "", "", fmt.Errorf("no location header in redirect response")
				}
			} else {
				return "", "", "", fmt.Errorf("expected redirect (307) or OK (200), got status: %d", headResp.StatusCode)
			}

			// The bottle digest is the SHA256 hash
			sha256 := bottleDigest

			return bottleDigest, downloadURL, sha256, nil
		}
	}

	return "", "", "", fmt.Errorf("no bottle found for platform: %s", platform)
}

// downloadBottle downloads the bottle tarball
func (pm *PackageManager) downloadBottle(ctx context.Context, url, destPath string) error {
	pm.logger.Printf("Downloading bottle from: %s", url)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Create output file
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	var resp *http.Response

	// Download
	// Check if we are downloading directly from GHCR (requires Auth)
	// or from a signed redirect URL (does NOT allow Auth)
	if strings.Contains(url, "ghcr.io") {
		headers := map[string]string{
			"Authorization": "Bearer QQ==",
		}
		resp, err = pm.client.GetWithHeaders(ctx, url, headers)
	} else {
		resp, err = pm.client.Get(ctx, url)
	}

	if err != nil {
		pm.logger.Printf("✗ Failed to download bottle: %v", err)
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	pm.logger.Printf("✓ Downloaded %d bytes to %s", written, destPath)
	return nil
}

// verifyFileHash verifies the SHA256 hash of a downloaded file
func (pm *PackageManager) verifyFileHash(filePath, expectedHash string) error {
	pm.logger.Printf("Computing SHA256 hash of: %s", filePath)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("computing hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	pm.logger.Printf("  Expected: %s", expectedHash)
	pm.logger.Printf("  Actual:   %s", actualHash)

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	pm.logger.Printf("  ✓ Hashes match!")
	return nil
}

// extractBottle extracts a bottle tarball
func (pm *PackageManager) extractBottle(bottlePath, cellarPath string) error {
	pm.logger.Printf("Extracting bottle: %s -> %s", bottlePath, cellarPath)

	// Open the tarball
	f, err := os.Open(bottlePath)
	if err != nil {
		return fmt.Errorf("opening bottle: %w", err)
	}
	defer f.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Track statistics
	fileCount := 0
	dirCount := 0
	symlinkCount := 0

	// Extract each entry
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Construct target path
		targetPath := filepath.Join(cellarPath, header.Name)

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, err)
			}
			dirCount++
			pm.logger.Printf("  📁 %s/", header.Name)

		case tar.TypeSymlink:
			// Create symlink
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory for symlink: %w", err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Ignore if symlink already exists
				if !os.IsExist(err) {
					return fmt.Errorf("creating symlink %s -> %s: %w", targetPath, header.Linkname, err)
				}
			}
			symlinkCount++
			pm.logger.Printf("  🔗 %s -> %s", header.Name, header.Linkname)

		case tar.TypeReg:
			// Regular file
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}

			// Create and write file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("creating file %s: %w", targetPath, err)
			}

			written, err := io.Copy(outFile, tr)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("writing file %s: %w", targetPath, err)
			}

			if written != header.Size {
				return fmt.Errorf("file size mismatch for %s: expected %d, got %d", targetPath, header.Size, written)
			}

			fileCount++
			execFlag := ""
			if header.Mode&0111 != 0 {
				execFlag = " (executable)"
			}
			pm.logger.Printf("  📄 %s (%d bytes)%s", header.Name, header.Size, execFlag)

		default:
			pm.logger.Printf("  ⚠️  Skipping unsupported file type %v for %s", header.Typeflag, header.Name)
		}
	}

	pm.logger.Printf("✓ Extraction complete:")
	pm.logger.Printf("  - %d files", fileCount)
	pm.logger.Printf("  - %d directories", dirCount)
	pm.logger.Printf("  - %d symlinks", symlinkCount)

	return nil
}