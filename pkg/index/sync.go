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
	RepoURL    = "https://github.com/arc-language/upkg"
	RepoBranch = "main"
)

// Sync clones the repo and copies the 3 things we need into the cache
func Sync(cacheDir string) error {
	tempDir, err := os.MkdirTemp("", "upkg-clone-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Updating package index from %s...\n", RepoURL)

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

	indexDir := filepath.Join(cacheDir, "index")
	os.MkdirAll(indexDir, 0755)

	// 1. Nix index
	if err := copyFile(
		filepath.Join(tempDir, "packages", "nix", "x86_64_linux.json"),
		filepath.Join(indexDir, "nix_x86_64_linux.json"),
	); err != nil {
		fmt.Printf("Warning: nix index: %v\n", err)
	}

	// 2. Winget index
	if err := copyFile(
		filepath.Join(tempDir, "packages", "winget", "default.json"),
		filepath.Join(indexDir, "winget_default.json"),
	); err != nil {
		fmt.Printf("Warning: winget index: %v\n", err)
	}

	// 3. deps/ registry
	if err := copyDir(
		filepath.Join(tempDir, "deps"),
		filepath.Join(cacheDir, "deps"),
	); err != nil {
		fmt.Printf("Warning: deps registry: %v\n", err)
	}

	fmt.Println("Package index updated successfully.")
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	os.MkdirAll(dst, 0755)

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}