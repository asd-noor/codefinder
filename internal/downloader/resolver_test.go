package downloader

import (
	"context"
	"testing"
	"time"
)

func TestGitHubResolver(t *testing.T) {
	resolver := NewGitHubResolver("golang", "tools", "")
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	version, err := resolver.ResolveLatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to resolve version: %v", err)
	}
	
	if version == "" {
		t.Error("Expected non-empty version")
	}
	
	t.Logf("Latest gopls version: %s", version)
}

func TestNPMResolver(t *testing.T) {
	resolver := NewNPMResolver("typescript-language-server")
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	version, err := resolver.ResolveLatestVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to resolve version: %v", err)
	}
	
	if version == "" {
		t.Error("Expected non-empty version")
	}
	
	t.Logf("Latest typescript-language-server version: %s", version)
}

func TestDynamicVersionResolution(t *testing.T) {
	tests := []struct {
		lang string
		name string
	}{
		{"go", "gopls"},
		{"typescript", "typescript-language-server"},
		{"python", "pyright"},
		{"lua", "lua-language-server"},
		{"zig", "zls"},
	}
	
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			metadata, err := GetLSPMetadata(tt.lang)
			if err != nil {
				t.Fatalf("Failed to get metadata: %v", err)
			}
			
			if metadata.Version == "" {
				t.Error("Expected non-empty version")
			}
			
			// Check that URL template was substituted
			for platform, url := range metadata.DownloadURLs {
				if url == "" {
					t.Errorf("Empty URL for platform %s", platform)
				}
				if containsPlaceholder(url) {
					t.Errorf("URL still contains {version} placeholder for platform %s: %s", platform, url)
				}
			}
			
			t.Logf("%s version: %s", tt.name, metadata.Version)
		})
	}
}

func containsPlaceholder(s string) bool {
	return len(s) > 0 && (s[0] == '{' || s[len(s)-1] == '}' || 
		len(s) > 8 && s[len(s)-9:] == "{version}")
}
