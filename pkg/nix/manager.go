// manager.go
package nix

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
	"zombiezen.com/go/nix/nar"
)

// NewPackageManager creates a new Nix package manager
func NewPackageManager(cfg *Config) *PackageManager {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.CacheURL == "" {
		cfg.CacheURL = DefaultCacheURL
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = DefaultInstallPath
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
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
		pm.logger.Printf("Initialized PackageManager")
		pm.logger.Printf("  CacheURL: %s", cfg.CacheURL)
		pm.logger.Printf("  InstallPath: %s", cfg.InstallPath)
		pm.logger.Printf("  Timeout: %s", cfg.Timeout)
	}

	return pm
}

// parseStorePath parses a StorePath field into a map of outputs
// Format: "bin=/nix/store/hash-name;dev=/nix/store/hash-name;/nix/store/hash-name"
func parseStorePath(storePath string) map[string]string {
	outputs := make(map[string]string)
	parts := strings.Split(storePath, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "=") {
			// Named output: "bin=/nix/store/hash-name"
			kv := strings.SplitN(part, "=", 2)
			outputName := kv[0]
			fullPath := kv[1]
			
			// Extract hash from "/nix/store/hash-name-version"
			pathWithoutPrefix := strings.TrimPrefix(fullPath, "/nix/store/")
			hashParts := strings.SplitN(pathWithoutPrefix, "-", 2)
			if len(hashParts) > 0 {
				outputs[outputName] = hashParts[0]
			}
		} else {
			// Default output: "/nix/store/hash-name"
			pathWithoutPrefix := strings.TrimPrefix(part, "/nix/store/")
			hashParts := strings.SplitN(pathWithoutPrefix, "-", 2)
			if len(hashParts) > 0 {
				outputs["out"] = hashParts[0]
			}
		}
	}

	return outputs
}

// LookupPackage finds a package by attribute name
func (pm *PackageManager) LookupPackage(attribute string) (*Package, error) {
	pkg, ok := x86_64_linux_Packages[attribute]
	if !ok {
		return nil, fmt.Errorf("package '%s' not found in registry", attribute)
	}
	return &pkg, nil
}

// Download downloads and installs a Nix package by attribute name
func (pm *PackageManager) Download(ctx context.Context, attribute string, opts *DownloadOptions) error {
	pm.logger.Printf("Starting download request for: %s", attribute)

	if opts == nil {
		opts = &DownloadOptions{}
	}

	// Set defaults
	if opts.Compression == "" {
		opts.Compression = CompressionXZ
	}
	if !opts.KeepArchive {
		opts.Extract = true
	}
	opts.VerifyHash = true

	// 1. Lookup package in registry
	pkg, err := pm.LookupPackage(attribute)
	if err != nil {
		return fmt.Errorf("looking up package: %w", err)
	}

	pm.logger.Printf("Found package: %s (%s)", pkg.Attribute, pkg.NameVersion)

	// 2. Parse store paths to get outputs
	allOutputs := parseStorePath(pkg.StorePath)
	if len(allOutputs) == 0 {
		return fmt.Errorf("no valid outputs found in store path")
	}

	pm.logger.Printf("Available outputs: %v", getKeys(allOutputs))

	// 3. Determine which outputs to download
	outputsToDownload := allOutputs
	if len(opts.Outputs) > 0 {
		// Filter to only requested outputs
		outputsToDownload = make(map[string]string)
		for _, requestedOutput := range opts.Outputs {
			if hash, ok := allOutputs[requestedOutput]; ok {
				outputsToDownload[requestedOutput] = hash
			} else {
				return fmt.Errorf("requested output '%s' not available (available: %v)", 
					requestedOutput, getKeys(allOutputs))
			}
		}
	}

	pm.logger.Printf("Downloading %d output(s): %v", len(outputsToDownload), getKeys(outputsToDownload))

	// 4. Create target directory (merge all outputs into one folder)
	targetDir := filepath.Join(pm.config.InstallPath, pkg.NameVersion)

	// 5. Download each output
	for outputName, storeHash := range outputsToDownload {
		pm.logger.Printf("--- Processing output: %s (%s) ---", outputName, storeHash)

		// A. Get Metadata
		narInfo, err := pm.GetNARInfo(ctx, storeHash)
		if err != nil {
			return fmt.Errorf("getting narinfo for %s: %w", outputName, err)
		}

		// B. Determine archive path
		archiveName := fmt.Sprintf("%s-%s.nar.%s", pkg.NameVersion, outputName, narInfo.Compression)
		narPath := filepath.Join(pm.config.InstallPath, archiveName)

		// C. Download
		if err := pm.downloadNAR(ctx, narInfo, narPath); err != nil {
			return fmt.Errorf("downloading %s: %w", outputName, err)
		}

		// D. Verify
		if opts.VerifyHash {
			if err := pm.verifyFileHash(narPath, narInfo.FileHash); err != nil {
				return fmt.Errorf("hash verification failed for %s: %w", outputName, err)
			}
		}

		// E. Extract (Merge into same directory)
		if opts.Extract {
			pm.logger.Printf("Extracting %s to: %s", outputName, targetDir)
			if err := pm.extractNAR(narPath, targetDir, narInfo.Compression); err != nil {
				return fmt.Errorf("extracting %s: %w", outputName, err)
			}

			// F. Cleanup
			if !opts.KeepArchive {
				os.Remove(narPath)
			}
		}
	}

	pm.logger.Printf("✓ All outputs downloaded and merged into: %s", targetDir)
	return nil
}

// GetNARInfo retrieves metadata for a store path
func (pm *PackageManager) GetNARInfo(ctx context.Context, storeHash string) (*NARInfo, error) {
	url := fmt.Sprintf("%s/%s.narinfo", pm.config.CacheURL, storeHash)
	pm.logger.Printf("Fetching NAR info from: %s", url)

	content, err := pm.client.GetString(ctx, url)
	if err != nil {
		pm.logger.Printf("✗ Failed to fetch NAR info: %v", err)
		return nil, err
	}

	narInfo, err := parseNARInfo(content)
	if err != nil {
		pm.logger.Printf("✗ Failed to parse NAR info: %v", err)
		return nil, err
	}

	return narInfo, nil
}

// downloadNAR downloads the NAR archive
func (pm *PackageManager) downloadNAR(ctx context.Context, narInfo *NARInfo, destPath string) error {
	url := fmt.Sprintf("%s/%s", pm.config.CacheURL, narInfo.URL)
	pm.logger.Printf("Downloading NAR from: %s", url)

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

	// Download
	if err := pm.client.Download(ctx, url, f); err != nil {
		pm.logger.Printf("✗ Failed to download NAR: %v", err)
		return fmt.Errorf("downloading: %w", err)
	}

	pm.logger.Printf("✓ Downloaded %d bytes to %s", narInfo.FileSize, destPath)
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

	actualHashBytes := hasher.Sum(nil)
	actualHashBase32 := toNixBase32(actualHashBytes)

	if actualHashBase32 != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHashBase32)
	}

	pm.logger.Printf("  ✓ Hashes match!")
	return nil
}

// extractNAR extracts a NAR archive
func (pm *PackageManager) extractNAR(narPath, destPath, compression string) error {
	pm.logger.Printf("Extracting NAR: %s -> %s (compression: %s)", narPath, destPath, compression)

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Decompress first if needed
	decompressedPath := narPath
	if compression != "none" {
		var err error
		decompressedPath, err = pm.decompressFile(narPath, compression)
		if err != nil {
			return fmt.Errorf("decompressing: %w", err)
		}
		defer os.Remove(decompressedPath)
	}

	// Extract the plain NAR
	return pm.extractPlainNAR(decompressedPath, destPath)
}

// decompressFile decompresses a file and returns the path to the decompressed file
func (pm *PackageManager) decompressFile(compressedPath, compression string) (string, error) {
	pm.logger.Printf("Decompressing %s archive...", compression)

	decompressedPath := compressedPath
	switch compression {
	case "xz":
		decompressedPath = compressedPath[:len(compressedPath)-3]
		return decompressedPath, pm.decompressXZ(compressedPath, decompressedPath)
	case "bzip2":
		decompressedPath = compressedPath[:len(compressedPath)-4]
		return decompressedPath, pm.decompressBZip2(compressedPath, decompressedPath)
	default:
		return "", fmt.Errorf("unsupported compression: %s", compression)
	}
}

// decompressXZ decompresses an xz file
func (pm *PackageManager) decompressXZ(src, dst string) error {
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer inputFile.Close()

	xzReader, err := xz.NewReader(inputFile)
	if err != nil {
		return fmt.Errorf("creating xz reader: %w", err)
	}

	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, xzReader); err != nil {
		return fmt.Errorf("decompressing data: %w", err)
	}

	return nil
}

// decompressBZip2 decompresses a bzip2 file
func (pm *PackageManager) decompressBZip2(src, dst string) error {
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer inputFile.Close()

	bzReader := bzip2.NewReader(inputFile)

	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, bzReader); err != nil {
		return fmt.Errorf("decompressing data: %w", err)
	}

	return nil
}

// extractPlainNAR extracts an uncompressed NAR archive
func (pm *PackageManager) extractPlainNAR(narPath, destPath string) error {
	pm.logger.Printf("Extracting NAR archive...")

	f, err := os.Open(narPath)
	if err != nil {
		return fmt.Errorf("opening NAR file: %w", err)
	}
	defer f.Close()

	bufReader := bufio.NewReader(f)
	narReader := nar.NewReader(bufReader)

	fileCount := 0

	for {
		hdr, err := narReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading NAR entry: %w", err)
		}

		targetPath := filepath.Join(destPath, hdr.Path)

		switch hdr.Mode.Type() {
		case os.ModeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, err)
			}
		case os.ModeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}
			if err := os.Symlink(hdr.LinkTarget, targetPath); err != nil {
				return fmt.Errorf("creating symlink: %w", err)
			}
		case 0: // Regular file
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}

			perm := os.FileMode(0644)
			if hdr.Mode&0111 != 0 {
				perm = 0755
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return fmt.Errorf("creating file %s: %w", targetPath, err)
			}

			written, err := io.Copy(outFile, narReader)
			outFile.Close()
			if err != nil {
				return fmt.Errorf("writing file: %w", err)
			}
			if written != hdr.Size {
				return fmt.Errorf("size mismatch")
			}
			fileCount++
		}
	}

	pm.logger.Printf("✓ Extraction complete (%d files)", fileCount)
	return nil
}

// Helper function to get map keys
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}