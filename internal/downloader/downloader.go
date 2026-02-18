package downloader

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

// Downloader handles LSP binary downloads and caching.
type Downloader struct {
	cacheDir string
	client   *http.Client
}

// New creates a new Downloader with the default cache directory.
func New() (*Downloader, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache dir: %w", err)
	}
	return &Downloader{
		cacheDir: cacheDir,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

// GetCacheDir returns the cache directory for LSP binaries.
// Priority: $CODEMAP_CACHE_DIR -> $XDG_CACHE_HOME/codemap/lsp -> ~/.cache/codemap/lsp
func GetCacheDir() (string, error) {
	if dir := os.Getenv("CODEMAP_CACHE_DIR"); dir != "" {
		return filepath.Join(dir, "lsp"), nil
	}

	if runtime.GOOS != "windows" {
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "codemap", "lsp"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Local", "codemap", "lsp"), nil
	}

	return filepath.Join(home, ".cache", "codemap", "lsp"), nil
}

// EnsureLSP ensures the LSP binary for the given language is available.
// Returns the path to the binary. Priority:
// 1. customPath (if provided and exists)
// 2. System PATH
// 3. Cache directory (download if needed)
func (d *Downloader) EnsureLSP(ctx context.Context, lang, customPath string) (string, error) {
	metadata, err := GetLSPMetadata(lang)
	if err != nil {
		return "", err
	}

	// Priority 1: Custom path from flags
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			log.Printf("[%s] Using custom LSP path: %s", lang, customPath)
			return customPath, nil
		}
		log.Printf("[%s] Custom path not found: %s, falling back...", lang, customPath)
	}

	// Priority 2: System PATH
	if systemPath, err := findInPath(metadata.BinaryName); err == nil {
		log.Printf("[%s] Using system LSP: %s", lang, systemPath)
		return systemPath, nil
	}

	// Priority 3: Cache directory
	cachedPath := d.getCachedBinaryPath(lang, metadata.Version)
	if _, err := os.Stat(cachedPath); err == nil {
		log.Printf("[%s] Using cached LSP: %s", lang, cachedPath)
		return cachedPath, nil
	}

	// Download needed
	log.Printf("[%s] LSP not found, downloading %s %s...", lang, metadata.Name, metadata.Version)
	if err := d.downloadAndInstall(ctx, lang, metadata); err != nil {
		return "", fmt.Errorf("failed to download %s: %w", metadata.Name, err)
	}

	log.Printf("[%s] Successfully downloaded and installed %s %s", lang, metadata.Name, metadata.Version)
	return cachedPath, nil
}

// getCachedBinaryPath returns the expected path for a cached binary.
func (d *Downloader) getCachedBinaryPath(lang, version string) string {
	binaryName := lspMetadata[lang].BinaryName
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	return filepath.Join(d.cacheDir, lang, version, binaryName)
}

// downloadAndInstall downloads and installs an LSP binary.
func (d *Downloader) downloadAndInstall(ctx context.Context, lang string, metadata *LSPServerMetadata) error {
	platform := GetPlatformKey()
	downloadURL, ok := metadata.DownloadURLs[platform]
	if !ok {
		return fmt.Errorf("no download URL for platform: %s", platform)
	}

	// Create version directory
	versionDir := filepath.Join(d.cacheDir, lang, metadata.Version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("failed to create version dir: %w", err)
	}

	// Download to temporary file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("codemap-lsp-%s-*", lang))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if err := d.downloadFile(ctx, downloadURL, tmpFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum if provided
	if checksum := metadata.Checksums[platform]; checksum != "" {
		if err := verifyChecksum(tmpFile.Name(), checksum); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract archive
	if metadata.IsArchive {
		if err := d.extractArchive(tmpFile.Name(), versionDir, metadata, platform); err != nil {
			return fmt.Errorf("extraction failed: %w", err)
		}
	} else {
		// Direct binary
		binaryPath := d.getCachedBinaryPath(lang, metadata.Version)
		if err := copyFile(tmpFile.Name(), binaryPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	return nil
}

// downloadFile downloads a file with retries.
func (d *Downloader) downloadFile(ctx context.Context, url string, dest *os.File) error {
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

		resp, err := d.client.Do(req)
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

// extractArchive extracts an archive to the destination directory.
func (d *Downloader) extractArchive(archivePath, destDir string, metadata *LSPServerMetadata, platform string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return d.extractZip(archivePath, destDir, metadata)
	}
	return d.extractTarGz(archivePath, destDir, metadata)
}

// extractTarGz extracts a .tar.gz archive.
func (d *Downloader) extractTarGz(archivePath, destDir string, metadata *LSPServerMetadata) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Find and extract the binary
	targetPath := metadata.ArchivePath
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Check if this is the binary we want
		if strings.HasSuffix(header.Name, targetPath) || header.Name == targetPath {
			binaryPath := d.getCachedBinaryPath(getLanguageByBinary(metadata.BinaryName), metadata.Version)
			return extractFile(tr, binaryPath, header.FileInfo().Mode())
		}
	}

	return fmt.Errorf("binary not found in archive: %s", targetPath)
}

// extractZip extracts a .zip archive.
func (d *Downloader) extractZip(archivePath, destDir string, metadata *LSPServerMetadata) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	targetPath := metadata.ArchivePath
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, targetPath) || f.Name == targetPath {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			binaryPath := d.getCachedBinaryPath(getLanguageByBinary(metadata.BinaryName), metadata.Version)
			return extractFile(rc, binaryPath, f.Mode())
		}
	}

	return fmt.Errorf("binary not found in archive: %s", targetPath)
}

// extractFile extracts a single file from a reader.
func extractFile(r io.Reader, destPath string, mode os.FileMode) error {
	// Ensure directory exists
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

// findInPath searches for a binary in the system PATH.
func findInPath(binaryName string) (string, error) {
	// Add .exe extension on Windows
	if runtime.GOOS == "windows" && !strings.HasSuffix(binaryName, ".exe") {
		binaryName += ".exe"
	}

	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)

	for _, dir := range paths {
		fullPath := filepath.Join(dir, binaryName)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			// Check if executable on Unix-like systems
			if runtime.GOOS != "windows" {
				if info.Mode()&0111 == 0 {
					continue
				}
			}
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("%s not found in PATH", binaryName)
}

// getLanguageByBinary maps binary name back to language.
func getLanguageByBinary(binaryName string) string {
	for lang, meta := range lspMetadata {
		if meta.BinaryName == binaryName {
			return lang
		}
	}
	return ""
}
