// manager.go
package nix

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// ResolvePackageName queries search.nixos.org Elasticsearch API to find all outputs for a package.
// Returns: (map[outputName]storeHash, nameWithVersion, error)
func (pm *PackageManager) ResolvePackageName(ctx context.Context, packageName string, platform Platform) (map[string]string, string, error) {
	if platform == "" {
		var err error
		platform, err = DetectPlatform()
		if err != nil {
			return nil, "", err
		}
	}

	// Use search.nixos.org Elasticsearch API
	channel := "nixos-unstable" // Could make this configurable via Config
	searchURL := fmt.Sprintf("https://search.nixos.org/%s/_search", channel)
	pm.logger.Printf("Resolving package '%s' via search.nixos.org API: %s", packageName, searchURL)

	// Build Elasticsearch query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"type": "package"}},
					{"term": map[string]interface{}{"package_system": string(platform)}},
				},
				"should": []map[string]interface{}{
					{"match": map[string]interface{}{
						"package_attr_name": map[string]interface{}{
							"query": packageName,
							"boost": 3,
						},
					}},
					{"match": map[string]interface{}{
						"package_pname": map[string]interface{}{
							"query": packageName,
							"boost": 2,
						},
					}},
				},
				"minimum_should_match": 1,
			},
		},
		"size": 1, // Get the best match
	}

	// Marshal query to JSON
	jsonData, err := json.Marshal(query)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling query: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, "", fmt.Errorf("creating search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", pm.client.userAgent)

	// Execute request
	resp, err := pm.client.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("search API returned status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source struct {
					PackageAttrName string            `json:"package_attr_name"`
					PackagePname    string            `json:"package_pname"`
					PackageVersion  string            `json:"package_version"`
					PackageOutputs  map[string]string `json:"package_outputs"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("parsing search response: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, "", fmt.Errorf("package '%s' not found for platform '%s'", packageName, platform)
	}

	hit := result.Hits.Hits[0].Source
	pm.logger.Printf("Found package: %s (v%s)", hit.PackageAttrName, hit.PackageVersion)

	// Extract store hashes from full paths
	// Path format: /nix/store/<hash>-<name>-<version>[-<output>]
	outputs := make(map[string]string)
	for outputName, storePath := range hit.PackageOutputs {
		// Parse "/nix/store/hash-name-version" to get just "hash"
		pathWithoutPrefix := strings.TrimPrefix(storePath, "/nix/store/")
		parts := strings.SplitN(pathWithoutPrefix, "-", 2)
		if len(parts) > 0 && parts[0] != "" {
			outputs[outputName] = parts[0]
			pm.logger.Printf("  Output '%s': %s", outputName, parts[0])
		}
	}

	if len(outputs) == 0 {
		return nil, "", fmt.Errorf("no valid outputs found for package '%s'", packageName)
	}

	nameVersion := fmt.Sprintf("%s-%s", hit.PackagePname, hit.PackageVersion)
	pm.logger.Printf("✓ Resolved '%s' to %d outputs", packageName, len(outputs))

	return outputs, nameVersion, nil
}

// helper to print keys
func reflectKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Download downloads and installs a Nix package.
// If opts.StoreHash is empty, it resolves ALL outputs (bin, dev, lib, etc.) and downloads them all.
func (pm *PackageManager) Download(ctx context.Context, name, version string, opts *DownloadOptions) error {
	pm.logger.Printf("Starting download request for: %s", name)

	if opts == nil {
		opts = &DownloadOptions{}
	}

	// 1. Ensure Platform
	if opts.Platform == "" {
		detected, err := DetectPlatform()
		if err != nil {
			return fmt.Errorf("detecting platform: %w", err)
		}
		opts.Platform = detected
		pm.logger.Printf("Auto-detected platform: %s", opts.Platform)
	}

	// 2. Determine what to download
	// downloads map maps "outputName" -> "storeHash"
	downloads := make(map[string]string)
	var folderName string

	if opts.StoreHash != "" {
		// Manual mode: specific hash only
		downloads["default"] = opts.StoreHash
		if version == "" {
			folderName = name
		} else {
			folderName = fmt.Sprintf("%s-%s", name, version)
		}
	} else {
		// Auto-resolve mode: fetch all outputs
		pm.logger.Printf("StoreHash not provided. Attempting to auto-resolve package: %s", name)
		resolvedOutputs, resolvedName, err := pm.ResolvePackageName(ctx, name, opts.Platform)
		if err != nil {
			return fmt.Errorf("resolving package name: %w", err)
		}
		downloads = resolvedOutputs

		if version == "" {
			folderName = resolvedName
		} else {
			folderName = fmt.Sprintf("%s-%s", name, version)
		}
	}

	// Set defaults
	if opts.Compression == "" {
		opts.Compression = CompressionXZ
	}
	if !opts.Extract && opts.KeepArchive {
		opts.Extract = false
	} else if opts.Extract == false && opts.KeepArchive == false {
		opts.Extract = true
	}
	if opts.VerifyHash == false {
		opts.VerifyHash = true
	}

	pm.logger.Printf("Download options:")
	pm.logger.Printf("  Target Folder: %s", folderName)
	pm.logger.Printf("  Outputs to fetch: %d %v", len(downloads), reflectKeys(downloads))

	// 3. Process each output
	// We extract everything into the SAME folder to merge them (like installing headers and libs together)
	targetDir := filepath.Join(pm.config.InstallPath, folderName)

	for outputName, hash := range downloads {
		pm.logger.Printf("--- Processing output: %s (%s) ---", outputName, hash)

		// A. Get Metadata
		narInfo, err := pm.GetNARInfo(ctx, hash)
		if err != nil {
			return fmt.Errorf("getting narinfo for %s: %w", outputName, err)
		}

		// B. Determine archive path
		// We append the output name to the archive file so they don't overwrite each other
		archiveName := fmt.Sprintf("%s-%s.nar.%s", folderName, outputName, narInfo.Compression)
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

		// E. Extract (Merge)
		if opts.Extract {
			pm.logger.Printf("Extracting %s to merged directory: %s", outputName, targetDir)
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
		pm.logger.Printf("❌ Failed to fetch NAR info: %v", err)
		return nil, err
	}

	narInfo, err := parseNARInfo(content)
	if err != nil {
		pm.logger.Printf("❌ Failed to parse NAR info: %v", err)
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
		pm.logger.Printf("❌ Failed to download NAR: %v", err)
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
		// Clean up decompressed file after extraction
		defer os.Remove(decompressedPath)
	}

	// Now extract the plain NAR
	return pm.extractPlainNAR(decompressedPath, destPath)
}

// decompressFile decompresses a file and returns the path to the decompressed file
func (pm *PackageManager) decompressFile(compressedPath, compression string) (string, error) {
	pm.logger.Printf("Decompressing %s archive...", compression)

	// Output path without compression extension
	decompressedPath := compressedPath
	switch compression {
	case "xz":
		decompressedPath = compressedPath[:len(compressedPath)-3] // Remove .xz
	case "bzip2":
		decompressedPath = compressedPath[:len(compressedPath)-4] // Remove .bz2
	default:
		return "", fmt.Errorf("unsupported compression: %s", compression)
	}

	switch compression {
	case "xz":
		return decompressedPath, pm.decompressXZ(compressedPath, decompressedPath)
	case "bzip2":
		return decompressedPath, pm.decompressBZip2(compressedPath, decompressedPath)
	default:
		return "", fmt.Errorf("unsupported compression: %s", compression)
	}
}

// decompressXZ decompresses an xz file using native Go library
func (pm *PackageManager) decompressXZ(src, dst string) error {
	// Open source file
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer inputFile.Close()

	// Create XZ reader
	xzReader, err := xz.NewReader(inputFile)
	if err != nil {
		return fmt.Errorf("creating xz reader: %w", err)
	}

	// Create destination file
	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	// Copy content
	if _, err := io.Copy(outFile, xzReader); err != nil {
		return fmt.Errorf("decompressing data: %w", err)
	}

	return nil
}

// decompressBZip2 decompresses a bzip2 file using standard Go library
func (pm *PackageManager) decompressBZip2(src, dst string) error {
	// Open source file
	inputFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer inputFile.Close()

	// Create BZip2 reader (standard library)
	bzReader := bzip2.NewReader(inputFile)

	// Create destination file
	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	// Copy content
	if _, err := io.Copy(outFile, bzReader); err != nil {
		return fmt.Errorf("decompressing data: %w", err)
	}

	return nil
}

// extractPlainNAR extracts an uncompressed NAR archive
func (pm *PackageManager) extractPlainNAR(narPath, destPath string) error {
	pm.logger.Printf("Extracting NAR archive using Go NAR library...")

	// Open the NAR file
	f, err := os.Open(narPath)
	if err != nil {
		return fmt.Errorf("opening NAR file: %w", err)
	}
	defer f.Close()

	// Create a buffered reader for better performance
	bufReader := bufio.NewReader(f)
	narReader := nar.NewReader(bufReader)

	// Track statistics
	fileCount := 0

	// Read and extract each entry
	for {
		hdr, err := narReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading NAR entry: %w", err)
		}

		// Construct target path
		targetPath := filepath.Join(destPath, hdr.Path)

		// Handle different file types
		switch hdr.Mode.Type() {
		case os.ModeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, err)
			}
		case os.ModeSymlink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}
			// Create symlink
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

		default:
			// Ignore other types
		}
	}

	pm.logger.Printf("✓ Extraction complete (%d files)", fileCount)
	return nil
}