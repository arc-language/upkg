package index

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	RepoURL = "https://github.com/arc-language/upkg"
	RepoBranch = "main" // or master
)

// Sync ensures the package index JSONs are present in the cache
func Sync(cacheDir string) error {
	indexDir := filepath.Join(cacheDir, "index")
	
	// Create index directory if not exists
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return fmt.Errorf("creating index directory: %w", err)
	}

	// Create a temp directory for cloning
	tempDir, err := os.MkdirTemp("", "upkg-clone-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup repo after we are done

	fmt.Printf("Updating package index from %s...\n", RepoURL)

	// Clone the repository (depth 1 for speed)
	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:           RepoURL,
		ReferenceName: plumbing.NewBranchReferenceName(RepoBranch),
		SingleBranch:  true,
		Depth:         1,
		Progress:      os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Copy specific JSON files to cache
	// 1. Nix: packages/nix/x86_64_linux.json -> cache/index/nix_x86_64_linux.json
	if err := copyFile(
		filepath.Join(tempDir, "packages", "nix", "x86_64_linux.json"),
		filepath.Join(indexDir, "nix_x86_64_linux.json"),
	); err != nil {
		fmt.Printf("Warning: Failed to copy nix index: %v\n", err)
	}

	// 2. Winget: packages/winget/default.json -> cache/index/winget_default.json
	if err := copyFile(
		filepath.Join(tempDir, "packages", "winget", "default.json"),
		filepath.Join(indexDir, "winget_default.json"),
	); err != nil {
		fmt.Printf("Warning: Failed to copy winget index: %v\n", err)
	}

	fmt.Println("Package index updated successfully.")
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}