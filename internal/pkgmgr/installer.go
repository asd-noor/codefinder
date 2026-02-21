package pkgmgr

import (
	"archive/tar"
	"archive/zip"
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
)

// Installer handles downloading and installing packages.
type Installer struct {
	manager    *Manager
	httpClient *http.Client
}

// NewInstaller creates a new installer instance.
func NewInstaller(manager *Manager) *Installer {
	return &Installer{
		manager: manager,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Install downloads and installs a package.
func (i *Installer) Install(ctx context.Context, packageName string, metadata *LSPMetadata) error {
	// Check if already installed
	if installed, version, _ := i.manager.IsInstalled(packageName); installed {
		log.Printf("[%s] Already installed (version %s)", packageName, version)
		return nil
	}

	platform := GetPlatformKey()
	downloadURL, ok := metadata.DownloadURLs[platform]
	if !ok {
		return fmt.Errorf("no download URL for platform: %s", platform)
	}

	log.Printf("[%s] Installing version %s...", packageName, metadata.Version)

	// Create version directory
	versionDir := filepath.Join(i.manager.packagesDir, packageName, metadata.Version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("failed to create version directory: %w", err)
	}

	// Download to temporary file
	tmpFile, err := os.CreateTemp(i.manager.tmpDir, fmt.Sprintf("codemap-%s-*", packageName))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := i.downloadFile(ctx, downloadURL, tmpFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum if provided
	if checksum := metadata.Checksums[platform]; checksum != "" {
		if err := verifyChecksum(tmpFile.Name(), checksum); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract or copy binary
	var binaryPath string
	if metadata.IsArchive {
		binaryPath, err = i.extractArchive(tmpFile.Name(), versionDir, metadata, platform)
		if err != nil {
			return fmt.Errorf("extraction failed: %w", err)
		}
	} else {
		// Direct binary download
		binaryName := metadata.BinaryName
		if runtime.GOOS == "windows" && filepath.Ext(binaryName) != ".exe" {
			binaryName += ".exe"
		}
		binaryPath = filepath.Join(versionDir, binaryName)
		if err := copyFile(tmpFile.Name(), binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	// Write package metadata
	pkg := &Package{
		Name:        packageName,
		Version:     metadata.Version,
		BinaryName:  metadata.BinaryName,
		InstalledAt: time.Now().Format(time.RFC3339),
		DownloadURL: downloadURL,
		Checksum:    metadata.Checksums[platform],
	}
	if err := i.manager.writePackageMetadata(packageName, metadata.Version, pkg); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Create 'current' symlink to this version
	pkgDir := filepath.Join(i.manager.packagesDir, packageName)
	currentLink := filepath.Join(pkgDir, "current")
	_ = os.Remove(currentLink) // Remove existing
	if err := os.Symlink(metadata.Version, currentLink); err != nil {
		return fmt.Errorf("failed to create current version link: %w", err)
	}

	// Create binary symlink in bin directory
	binPath, err := GetBinaryPath(metadata.BinaryName)
	if err != nil {
		return err
	}
	if err := createSymlink(binaryPath, binPath); err != nil {
		return fmt.Errorf("failed to create binary symlink: %w", err)
	}

	log.Printf("[%s] Successfully installed version %s", packageName, metadata.Version)
	return nil
}

// downloadFile downloads a file with retries.
func (i *Installer) downloadFile(ctx context.Context, url string, dest *os.File) error {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Printf("Retry %d/%d after %v...", attempt, maxRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := i.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			continue
		}

		// Reset file position
		if _, err := dest.Seek(0, 0); err != nil {
			resp.Body.Close()
			return err
		}

		_, err = io.Copy(dest, resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}

// extractArchive extracts an archive and returns the path to the binary.
func (i *Installer) extractArchive(archivePath, destDir string, metadata *LSPMetadata, platform string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return i.extractZip(archivePath, destDir, metadata)
	}
	return i.extractTarGz(archivePath, destDir, metadata)
}

// extractTarGz extracts a .tar.gz archive.
func (i *Installer) extractTarGz(archivePath, destDir string, metadata *LSPMetadata) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	targetPath := metadata.ArchivePath
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read error: %w", err)
		}

		if strings.HasSuffix(header.Name, targetPath) || header.Name == targetPath {
			binaryName := metadata.BinaryName
			if runtime.GOOS == "windows" && filepath.Ext(binaryName) != ".exe" {
				binaryName += ".exe"
			}
			binaryPath := filepath.Join(destDir, binaryName)
			if err := extractFile(tr, binaryPath, header.FileInfo().Mode()); err != nil {
				return "", err
			}
			return binaryPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive: %s", targetPath)
}

// extractZip extracts a .zip archive.
func (i *Installer) extractZip(archivePath, destDir string, metadata *LSPMetadata) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	targetPath := metadata.ArchivePath
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, targetPath) || f.Name == targetPath {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			binaryName := metadata.BinaryName
			if runtime.GOOS == "windows" && filepath.Ext(binaryName) != ".exe" {
				binaryName += ".exe"
			}
			binaryPath := filepath.Join(destDir, binaryName)
			if err := extractFile(rc, binaryPath, f.Mode()); err != nil {
				return "", err
			}
			return binaryPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive: %s", targetPath)
}

// extractFile extracts a single file from a reader.
func extractFile(r io.Reader, destPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		return err
	}

	// Ensure executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return err
		}
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of a file.
func verifyChecksum(filePath, expectedChecksum string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(h.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// copyFile copies a file from src to dst.
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

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}

// GetPlatformKey returns the platform key for the current system.
func GetPlatformKey() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Normalize arch names
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	}

	return fmt.Sprintf("%s-%s", os, arch)
}
