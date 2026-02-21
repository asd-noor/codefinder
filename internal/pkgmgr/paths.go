package pkgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetCodeMapHome returns the root directory for CodeMap package management.
// Priority: $CODEMAP_HOME -> $XDG_CACHE_HOME/codemap -> ~/.cache/codemap (Unix) / %LOCALAPPDATA%\codemap (Windows)
func GetCodeMapHome() (string, error) {
	// Priority 1: CODEMAP_HOME environment variable
	if home := os.Getenv("CODEMAP_HOME"); home != "" {
		return home, nil
	}

	// Priority 2: XDG_CACHE_HOME on Unix-like systems
	if runtime.GOOS != "windows" {
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "codemap"), nil
		}
	}

	// Priority 3: Platform-specific defaults
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(userHome, "AppData", "Local", "codemap"), nil
	default:
		return filepath.Join(userHome, ".cache", "codemap"), nil
	}
}

// GetBinDir returns the unified bin directory containing all LSP executables.
func GetBinDir() (string, error) {
	home, err := GetCodeMapHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "bin"), nil
}

// GetPackagesDir returns the directory containing all installed packages.
func GetPackagesDir() (string, error) {
	home, err := GetCodeMapHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "packages"), nil
}

// GetRegistryDir returns the directory containing package registry metadata.
func GetRegistryDir() (string, error) {
	home, err := GetCodeMapHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "registry"), nil
}

// GetTmpDir returns the temporary directory for downloads.
func GetTmpDir() (string, error) {
	home, err := GetCodeMapHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "tmp"), nil
}

// GetPackageDir returns the directory for a specific package.
func GetPackageDir(packageName string) (string, error) {
	pkgDir, err := GetPackagesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(pkgDir, packageName), nil
}

// GetPackageVersionDir returns the directory for a specific package version.
func GetPackageVersionDir(packageName, version string) (string, error) {
	pkgDir, err := GetPackageDir(packageName)
	if err != nil {
		return "", err
	}
	return filepath.Join(pkgDir, version), nil
}

// GetBinaryPath returns the path to a binary in the unified bin directory.
func GetBinaryPath(binaryName string) (string, error) {
	binDir, err := GetBinDir()
	if err != nil {
		return "", err
	}
	
	// Add .exe on Windows
	if runtime.GOOS == "windows" && filepath.Ext(binaryName) != ".exe" {
		binaryName += ".exe"
	}
	
	return filepath.Join(binDir, binaryName), nil
}

// EnsureDirectories creates all required CodeMap directories if they don't exist.
func EnsureDirectories() error {
	dirs := []func() (string, error){
		GetCodeMapHome,
		GetBinDir,
		GetPackagesDir,
		GetRegistryDir,
		GetTmpDir,
	}

	for _, dirFunc := range dirs {
		dir, err := dirFunc()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
