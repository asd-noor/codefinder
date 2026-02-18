package downloader

import (
	"fmt"
	"runtime"
)

// LSPServerMetadata defines version and download information for an LSP server.
type LSPServerMetadata struct {
	Name         string
	Version      string
	BinaryName   string // name of the executable in the archive
	DownloadURLs map[string]string // platform -> download URL
	Checksums    map[string]string // platform -> SHA256 checksum
	IsArchive    bool              // whether download is an archive (tar.gz/zip)
	ArchivePath  string            // path to binary within archive (if applicable)
}

// GetLSPMetadata returns metadata for a given language's LSP server.
func GetLSPMetadata(lang string) (*LSPServerMetadata, error) {
	metadata, ok := lspMetadata[lang]
	if !ok {
		return nil, fmt.Errorf("no metadata for language: %s", lang)
	}
	return metadata, nil
}

// GetPlatformKey returns the platform identifier for the current system.
func GetPlatformKey() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// Pinned stable versions for each LSP server
var lspMetadata = map[string]*LSPServerMetadata{
	"go": {
		Name:       "gopls",
		Version:    "v0.17.1",
		BinaryName: "gopls",
		DownloadURLs: map[string]string{
			"linux-amd64":  "https://github.com/golang/tools/releases/download/gopls/v0.17.1/gopls-v0.17.1-linux-amd64.tar.gz",
			"linux-arm64":  "https://github.com/golang/tools/releases/download/gopls/v0.17.1/gopls-v0.17.1-linux-arm64.tar.gz",
			"darwin-amd64": "https://github.com/golang/tools/releases/download/gopls/v0.17.1/gopls-v0.17.1-darwin-amd64.tar.gz",
			"darwin-arm64": "https://github.com/golang/tools/releases/download/gopls/v0.17.1/gopls-v0.17.1-darwin-arm64.tar.gz",
			"windows-amd64": "https://github.com/golang/tools/releases/download/gopls/v0.17.1/gopls-v0.17.1-windows-amd64.zip",
		},
		Checksums: map[string]string{
			// These would be real checksums in production
			"linux-amd64":  "",
			"linux-arm64":  "",
			"darwin-amd64": "",
			"darwin-arm64": "",
			"windows-amd64": "",
		},
		IsArchive:   true,
		ArchivePath: "gopls",
	},
	"python": {
		Name:       "pyright",
		Version:    "1.1.390",
		BinaryName: "pyright-langserver",
		DownloadURLs: map[string]string{
			// Pyright is distributed via npm, we'll use a bundled approach
			// For now, use system installation as primary, download as fallback
			"linux-amd64":  "https://registry.npmjs.org/pyright/-/pyright-1.1.390.tgz",
			"linux-arm64":  "https://registry.npmjs.org/pyright/-/pyright-1.1.390.tgz",
			"darwin-amd64": "https://registry.npmjs.org/pyright/-/pyright-1.1.390.tgz",
			"darwin-arm64": "https://registry.npmjs.org/pyright/-/pyright-1.1.390.tgz",
			"windows-amd64": "https://registry.npmjs.org/pyright/-/pyright-1.1.390.tgz",
		},
		Checksums: map[string]string{
			"linux-amd64":  "",
			"linux-arm64":  "",
			"darwin-amd64": "",
			"darwin-arm64": "",
			"windows-amd64": "",
		},
		IsArchive:   true,
		ArchivePath: "package/langserver.index.js",
	},
	"typescript": {
		Name:       "typescript-language-server",
		Version:    "4.3.3",
		BinaryName: "typescript-language-server",
		DownloadURLs: map[string]string{
			"linux-amd64":  "https://registry.npmjs.org/typescript-language-server/-/typescript-language-server-4.3.3.tgz",
			"linux-arm64":  "https://registry.npmjs.org/typescript-language-server/-/typescript-language-server-4.3.3.tgz",
			"darwin-amd64": "https://registry.npmjs.org/typescript-language-server/-/typescript-language-server-4.3.3.tgz",
			"darwin-arm64": "https://registry.npmjs.org/typescript-language-server/-/typescript-language-server-4.3.3.tgz",
			"windows-amd64": "https://registry.npmjs.org/typescript-language-server/-/typescript-language-server-4.3.3.tgz",
		},
		Checksums: map[string]string{
			"linux-amd64":  "",
			"linux-arm64":  "",
			"darwin-amd64": "",
			"darwin-arm64": "",
			"windows-amd64": "",
		},
		IsArchive:   true,
		ArchivePath: "package/lib/cli.mjs",
	},
	"lua": {
		Name:       "lua-language-server",
		Version:    "3.13.3",
		BinaryName: "lua-language-server",
		DownloadURLs: map[string]string{
			"linux-amd64":  "https://github.com/LuaLS/lua-language-server/releases/download/3.13.3/lua-language-server-3.13.3-linux-x64.tar.gz",
			"linux-arm64":  "https://github.com/LuaLS/lua-language-server/releases/download/3.13.3/lua-language-server-3.13.3-linux-arm64.tar.gz",
			"darwin-amd64": "https://github.com/LuaLS/lua-language-server/releases/download/3.13.3/lua-language-server-3.13.3-darwin-x64.tar.gz",
			"darwin-arm64": "https://github.com/LuaLS/lua-language-server/releases/download/3.13.3/lua-language-server-3.13.3-darwin-arm64.tar.gz",
			"windows-amd64": "https://github.com/LuaLS/lua-language-server/releases/download/3.13.3/lua-language-server-3.13.3-win32-x64.zip",
		},
		Checksums: map[string]string{
			"linux-amd64":  "",
			"linux-arm64":  "",
			"darwin-amd64": "",
			"darwin-arm64": "",
			"windows-amd64": "",
		},
		IsArchive:   true,
		ArchivePath: "bin/lua-language-server",
	},
	"zig": {
		Name:       "zls",
		Version:    "0.14.0",
		BinaryName: "zls",
		DownloadURLs: map[string]string{
			"linux-amd64":  "https://github.com/zigtools/zls/releases/download/0.14.0/zls-linux-x86_64-0.14.0.tar.gz",
			"linux-arm64":  "https://github.com/zigtools/zls/releases/download/0.14.0/zls-linux-aarch64-0.14.0.tar.gz",
			"darwin-amd64": "https://github.com/zigtools/zls/releases/download/0.14.0/zls-macos-x86_64-0.14.0.tar.gz",
			"darwin-arm64": "https://github.com/zigtools/zls/releases/download/0.14.0/zls-macos-aarch64-0.14.0.tar.gz",
			"windows-amd64": "https://github.com/zigtools/zls/releases/download/0.14.0/zls-windows-x86_64-0.14.0.zip",
		},
		Checksums: map[string]string{
			"linux-amd64":  "",
			"linux-arm64":  "",
			"darwin-amd64": "",
			"darwin-arm64": "",
			"windows-amd64": "",
		},
		IsArchive:   true,
		ArchivePath: "zls",
	},
}
