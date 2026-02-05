package util

import (
	"path/filepath"
	"strings"
)

func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "file://" + path
	}
	return "file://" + convertToSlash(abs)
}

func URIToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return convertFromSlash(uri[7:])
	}
	return uri
}

func convertToSlash(path string) string {
	// Windows support if needed, but for now standard filepath
	return filepath.ToSlash(path)
}

func convertFromSlash(path string) string {
	return filepath.FromSlash(path)
}
