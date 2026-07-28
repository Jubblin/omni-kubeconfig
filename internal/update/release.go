package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultRepo is the GitHub repository used for releases.
	DefaultRepo = "Jubblin/omni-kubeconfig"

	defaultAPIBase      = "https://api.github.com"
	defaultDownloadBase = "https://github.com"
)

// Config controls update and install behavior.
type Config struct {
	Repo            string
	APIBaseURL      string
	DownloadBaseURL string
	HTTPClient      *http.Client
	CacheDir        string
	CacheTTL        time.Duration
}

func (c Config) withDefaults() Config {
	out := c
	if out.Repo == "" {
		out.Repo = DefaultRepo
	}
	if out.APIBaseURL == "" {
		out.APIBaseURL = defaultAPIBase
	}
	if out.DownloadBaseURL == "" {
		out.DownloadBaseURL = defaultDownloadBase
	}
	if out.HTTPClient == nil {
		out.HTTPClient = http.DefaultClient
	}
	if out.CacheTTL == 0 {
		out.CacheTTL = 24 * time.Hour
	}
	return out
}

func (c Config) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

// Release describes a GitHub release used for install/update.
type Release struct {
	TagName    string
	Version    string
	Prerelease bool
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
}

// FetchLatestStable returns the newest non-prerelease release from /releases/latest.
func FetchLatestStable(ctx context.Context, cfg Config) (Release, error) {
	cfg = cfg.withDefaults()
	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimRight(cfg.APIBaseURL, "/"), cfg.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := cfg.client().Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Release{}, fmt.Errorf("fetch latest release: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode release JSON: %w", err)
	}
	if payload.Prerelease {
		return Release{}, fmt.Errorf("latest release %q is a prerelease", payload.TagName)
	}
	if payload.TagName == "" {
		return Release{}, fmt.Errorf("release missing tag_name")
	}

	return Release{
		TagName:    payload.TagName,
		Version:    normalizeTag(payload.TagName),
		Prerelease: payload.Prerelease,
	}, nil
}

// ReleaseDownloadURL builds the asset download URL for a tag and asset file.
func ReleaseDownloadURL(cfg Config, tag, assetName string) string {
	cfg = cfg.withDefaults()
	base := strings.TrimRight(cfg.DownloadBaseURL, "/")
	return fmt.Sprintf("%s/%s/releases/download/%s/%s", base, cfg.Repo, tag, assetName)
}

func normalizeTag(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), "v")
}
