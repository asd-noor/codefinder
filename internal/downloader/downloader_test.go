package downloader

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetLSPMetadata(t *testing.T) {
	tests := []struct {
		lang      string
		wantError bool
	}{
		{"go", false},
		{"python", false},
		{"typescript", false},
		{"lua", false},
		{"zig", false},
		{"unsupported", true},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			meta, err := GetLSPMetadata(tt.lang)
			if tt.wantError {
				if err == nil {
					t.Error("expected error for unsupported language")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if meta.Name == "" {
				t.Error("expected non-empty name")
			}
			if meta.Version == "" {
				t.Error("expected non-empty version")
			}
			if meta.BinaryName == "" {
				t.Error("expected non-empty binary name")
			}
		})
	}
}

func TestGetPlatformKey(t *testing.T) {
	platform := GetPlatformKey()
	parts := strings.Split(platform, "-")
	if len(parts) != 2 {
		t.Errorf("expected format 'os-arch', got %s", platform)
	}

	expectedOS := runtime.GOOS
	expectedArch := runtime.GOARCH
	expected := expectedOS + "-" + expectedArch

	if platform != expected {
		t.Errorf("expected %s, got %s", expected, platform)
	}
}

func TestGetCacheDir(t *testing.T) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cacheDir == "" {
		t.Error("expected non-empty cache directory")
	}

	// Should end with lsp
	if !strings.HasSuffix(cacheDir, "lsp") {
		t.Errorf("expected cache dir to end with 'lsp', got %s", cacheDir)
	}
}

func TestMetadataHasPlatformURLs(t *testing.T) {
	languages := []string{"go", "python", "typescript", "lua", "zig"}
	platforms := []string{
		"linux-amd64",
		"linux-arm64",
		"darwin-amd64",
		"darwin-arm64",
		"windows-amd64",
	}

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			meta, err := GetLSPMetadata(lang)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, platform := range platforms {
				url, ok := meta.DownloadURLs[platform]
				if !ok {
					t.Errorf("missing download URL for platform %s", platform)
					continue
				}
				if url == "" {
					t.Errorf("empty download URL for platform %s", platform)
				}
				if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
					t.Errorf("invalid URL for platform %s: %s", platform, url)
				}
			}
		})
	}
}

func TestBinaryNameMapping(t *testing.T) {
	tests := []struct {
		lang       string
		binaryName string
	}{
		{"go", "gopls"},
		{"python", "pyright-langserver"},
		{"typescript", "typescript-language-server"},
		{"lua", "lua-language-server"},
		{"zig", "zls"},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			meta, err := GetLSPMetadata(tt.lang)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.BinaryName != tt.binaryName {
				t.Errorf("expected binary name %s, got %s", tt.binaryName, meta.BinaryName)
			}

			// Test reverse mapping
			lang := getLanguageByBinary(tt.binaryName)
			if lang != tt.lang {
				t.Errorf("expected language %s for binary %s, got %s", tt.lang, tt.binaryName, lang)
			}
		})
	}
}

func TestDownloaderCreation(t *testing.T) {
	dl, err := New()
	if err != nil {
		t.Fatalf("failed to create downloader: %v", err)
	}
	if dl.cacheDir == "" {
		t.Error("expected non-empty cache directory")
	}
	if dl.client == nil {
		t.Error("expected non-nil HTTP client")
	}
}
