package pkgmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// Manager handles package installation, updates, and lifecycle.
type Manager struct {
	packagesDir string
	binDir      string
	registryDir string
	tmpDir      string
}

// Package represents an installed package.
type Package struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	BinaryName   string `json:"binary_name"`
	InstalledAt  string `json:"installed_at"`
	DownloadURL  string `json:"download_url"`
	Checksum     string `json:"checksum"`
}

// NewManager creates a new package manager instance.
func NewManager() (*Manager, error) {
	// Ensure all directories exist
	if err := EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to initialize directories: %w", err)
	}

	packagesDir, err := GetPackagesDir()
	if err != nil {
		return nil, err
	}

	binDir, err := GetBinDir()
	if err != nil {
		return nil, err
	}

	registryDir, err := GetRegistryDir()
	if err != nil {
		return nil, err
	}

	tmpDir, err := GetTmpDir()
	if err != nil {
		return nil, err
	}

	return &Manager{
		packagesDir: packagesDir,
		binDir:      binDir,
		registryDir: registryDir,
		tmpDir:      tmpDir,
	}, nil
}

// IsInstalled checks if a package is installed.
func (m *Manager) IsInstalled(packageName string) (bool, string, error) {
	pkgDir := filepath.Join(m.packagesDir, packageName)
	
	// Check if current symlink exists
	currentLink := filepath.Join(pkgDir, "current")
	target, err := os.Readlink(currentLink)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("failed to read current version link: %w", err)
	}

	// Extract version from symlink target
	version := filepath.Base(target)
	
	// Verify the version directory exists
	versionDir := filepath.Join(pkgDir, version)
	if _, err := os.Stat(versionDir); err != nil {
		return false, "", nil
	}

	return true, version, nil
}

// ListInstalled returns all installed packages.
func (m *Manager) ListInstalled() ([]Package, error) {
	entries, err := os.ReadDir(m.packagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Package{}, nil
		}
		return nil, fmt.Errorf("failed to read packages directory: %w", err)
	}

	var packages []Package
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Read metadata
		pkg, err := m.readPackageMetadata(entry.Name())
		if err != nil {
			log.Printf("Warning: failed to read metadata for %s: %v", entry.Name(), err)
			continue
		}

		packages = append(packages, *pkg)
	}

	return packages, nil
}

// Uninstall removes a package and its binary symlink.
func (m *Manager) Uninstall(ctx context.Context, packageName string) error {
	installed, version, err := m.IsInstalled(packageName)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("package not installed: %s", packageName)
	}

	log.Printf("Uninstalling %s version %s...", packageName, version)

	// Remove binary symlink
	pkg, err := m.readPackageMetadata(packageName)
	if err == nil {
		binPath, _ := GetBinaryPath(pkg.BinaryName)
		if err := removeSymlink(binPath); err != nil {
			log.Printf("Warning: failed to remove binary symlink: %v", err)
		}
	}

	// Remove package directory
	pkgDir := filepath.Join(m.packagesDir, packageName)
	if err := os.RemoveAll(pkgDir); err != nil {
		return fmt.Errorf("failed to remove package directory: %w", err)
	}

	log.Printf("Successfully uninstalled %s", packageName)
	return nil
}

// GetBinaryPath returns the path to an installed package's binary.
func (m *Manager) GetBinaryPath(packageName string) (string, error) {
	pkg, err := m.readPackageMetadata(packageName)
	if err != nil {
		return "", err
	}

	return GetBinaryPath(pkg.BinaryName)
}

// readPackageMetadata reads the metadata for an installed package.
func (m *Manager) readPackageMetadata(packageName string) (*Package, error) {
	pkgDir := filepath.Join(m.packagesDir, packageName)
	currentLink := filepath.Join(pkgDir, "current")
	
	target, err := os.Readlink(currentLink)
	if err != nil {
		return nil, fmt.Errorf("package not installed or corrupted: %w", err)
	}

	version := filepath.Base(target)
	metadataPath := filepath.Join(pkgDir, version, ".metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &pkg, nil
}

// writePackageMetadata writes metadata for an installed package.
func (m *Manager) writePackageMetadata(packageName, version string, pkg *Package) error {
	versionDir := filepath.Join(m.packagesDir, packageName, version)
	metadataPath := filepath.Join(versionDir, ".metadata.json")

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// createSymlink creates a symlink or shim for the binary.
func createSymlink(source, target string) error {
	// Remove existing symlink if present
	_ = os.Remove(target)

	if runtime.GOOS == "windows" {
		// On Windows, create a .bat shim instead of symlink
		return createWindowsShim(source, target)
	}

	// Unix-like: create symlink
	return os.Symlink(source, target)
}

// removeSymlink removes a symlink or shim.
func removeSymlink(path string) error {
	if runtime.GOOS == "windows" {
		// Remove .bat file
		if err := os.Remove(path + ".bat"); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Remove(path)
}

// createWindowsShim creates a .bat file that calls the actual binary.
func createWindowsShim(binaryPath, shimPath string) error {
	batContent := fmt.Sprintf("@echo off\r\n\"%s\" %%*\r\n", binaryPath)
	return os.WriteFile(shimPath+".bat", []byte(batContent), 0755)
}

// AddToPath adds the bin directory to the environment PATH for the current process.
func (m *Manager) AddToPath() error {
	currentPath := os.Getenv("PATH")
	
	// Check if already in PATH
	if filepath.SplitList(currentPath) != nil {
		for _, p := range filepath.SplitList(currentPath) {
			if p == m.binDir {
				return nil // Already in PATH
			}
		}
	}

	// Add to PATH
	newPath := m.binDir + string(os.PathListSeparator) + currentPath
	return os.Setenv("PATH", newPath)
}
