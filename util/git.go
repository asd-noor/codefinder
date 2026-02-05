package util

import (
	"os"
	"path/filepath"
)

// FindGitRoot finds the root of the git repository starting from the current directory.
// Returns the current directory if .git is not found.
func FindGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			cwd, _ := os.Getwd()
			return cwd, nil
		}
		dir = parent
	}
}
