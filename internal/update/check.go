package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

// IsDevVersion reports whether version strings from dev or snapshot builds should skip auto-update.
func IsDevVersion(version string) bool {
	v := strings.ToLower(strings.TrimSpace(version))
	if v == "" {
		return true
	}
	if strings.Contains(v, "-dev") || strings.Contains(v, "-snapshot") || strings.Contains(v, "snapshot") {
		return true
	}
	if strings.Contains(v, "-dirty") || strings.Contains(v, "+dirty") {
		return true
	}
	// git describe forms like 0.3.0-5-gabc1234
	if strings.Count(v, "-") >= 2 {
		return true
	}
	return false
}

// IsNewerStable reports whether latest is strictly greater than current (semver tolerant).
func IsNewerStable(current, latest string) (bool, error) {
	cur, err := semver.ParseTolerant(normalizeTag(current))
	if err != nil {
		return false, fmt.Errorf("parse current version %q: %w", current, err)
	}
	lat, err := semver.ParseTolerant(normalizeTag(latest))
	if err != nil {
		return false, fmt.Errorf("parse latest version %q: %w", latest, err)
	}
	return lat.GT(cur), nil
}

type cacheRecord struct {
	CheckedAt      time.Time `json:"checked_at"`
	LatestTag      string    `json:"latest_tag"`
	LatestVersion  string    `json:"latest_version"`
	CurrentVersion string    `json:"current_version"`
}

func cacheFilePath(cfg Config) (string, error) {
	cfg = cfg.withDefaults()
	if cfg.CacheDir != "" {
		return filepath.Join(cfg.CacheDir, "update-check.json"), nil
	}
	dir, err := defaultCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-check.json"), nil
}

func defaultCacheDir() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "omni-kubeconfig"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "omni-kubeconfig"), nil
}

func readCache(cfg Config) (cacheRecord, bool, error) {
	path, err := cacheFilePath(cfg)
	if err != nil {
		return cacheRecord{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cacheRecord{}, false, nil
		}
		return cacheRecord{}, false, err
	}
	var rec cacheRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return cacheRecord{}, false, nil
	}
	return rec, true, nil
}

func writeCache(cfg Config, rec cacheRecord) error {
	path, err := cacheFilePath(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func cacheFresh(rec cacheRecord, ttl time.Duration, current string) bool {
	if rec.CheckedAt.IsZero() {
		return false
	}
	if normalizeTag(rec.CurrentVersion) != normalizeTag(current) {
		return false
	}
	return time.Since(rec.CheckedAt) < ttl
}
