package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VersionResolver fetches the latest version for an LSP server.
type VersionResolver interface {
	ResolveLatestVersion(ctx context.Context) (string, error)
}

// GitHubReleaseResolver resolves versions from GitHub releases.
type GitHubReleaseResolver struct {
	owner      string
	repo       string
	tagPrefix  string // optional prefix like "gopls/" for gopls releases
	httpClient *http.Client
}

// NPMResolver resolves versions from npm registry.
type NPMResolver struct {
	packageName string
	httpClient  *http.Client
}

// NewGitHubResolver creates a resolver for GitHub releases.
func NewGitHubResolver(owner, repo, tagPrefix string) *GitHubReleaseResolver {
	return &GitHubReleaseResolver{
		owner:     owner,
		repo:      repo,
		tagPrefix: tagPrefix,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewNPMResolver creates a resolver for npm packages.
func NewNPMResolver(packageName string) *NPMResolver {
	return &NPMResolver{
		packageName: packageName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ResolveLatestVersion fetches the latest GitHub release version.
func (r *GitHubReleaseResolver) ResolveLatestVersion(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", r.owner, r.repo)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GitHub release: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}
	
	var release struct {
		TagName string `json:"tag_name"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode GitHub response: %w", err)
	}
	
	// Return the full tag (with prefix if present)
	return release.TagName, nil
}

// ResolveLatestVersion fetches the latest npm package version.
func (r *NPMResolver) ResolveLatestVersion(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s/latest", r.packageName)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch npm package: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("npm registry returned %d: %s", resp.StatusCode, string(body))
	}
	
	var pkg struct {
		Version string `json:"version"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return "", fmt.Errorf("failed to decode npm response: %w", err)
	}
	
	return pkg.Version, nil
}
