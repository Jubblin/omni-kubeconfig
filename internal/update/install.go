package update

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

// CheckOptions controls update availability checks and prompts.
type CheckOptions struct {
	SkipCheck  bool
	ForceCheck bool
	NoPrompt   bool
}

// CheckResult describes whether an update is available.
type CheckResult struct {
	Current string
	Latest  Release
	Newer   bool
}

// Check reports whether a newer stable release exists.
func Check(ctx context.Context, cfg Config, currentVersion string, opts CheckOptions) (CheckResult, error) {
	cfg = cfg.withDefaults()
	result := CheckResult{Current: currentVersion}

	if opts.SkipCheck || IsDevVersion(currentVersion) {
		return result, nil
	}

	if !opts.ForceCheck {
		if rec, ok, err := readCache(cfg); err == nil && ok && cacheFresh(rec, cfg.CacheTTL, currentVersion) {
			result.Latest = Release{TagName: rec.LatestTag, Version: rec.LatestVersion}
			newer, err := IsNewerStable(currentVersion, rec.LatestVersion)
			if err != nil {
				return result, err
			}
			result.Newer = newer
			return result, nil
		}
	}

	latest, err := FetchLatestStable(ctx, cfg)
	if err != nil {
		return result, err
	}
	result.Latest = latest

	newer, err := IsNewerStable(currentVersion, latest.Version)
	if err != nil {
		return result, err
	}
	result.Newer = newer

	_ = writeCache(cfg, cacheRecord{
		CheckedAt:      time.Now().UTC(),
		LatestTag:      latest.TagName,
		LatestVersion:  latest.Version,
		CurrentVersion: currentVersion,
	})

	return result, nil
}

// MaybePrompt checks for updates and optionally installs when the user confirms.
func MaybePrompt(ctx context.Context, cfg Config, currentVersion string, opts CheckOptions, stdin io.Reader, stderr io.Writer) error {
	if opts.SkipCheck || shouldSkipEnv() {
		return nil
	}

	result, err := Check(ctx, cfg, currentVersion, opts)
	if err != nil || !result.Newer {
		return err
	}

	if opts.NoPrompt || !isInteractive(stdin) {
		return nil
	}

	fmt.Fprintf(stderr, "A new version is available: %s (you have %s).\n", result.Latest.Version, normalizeTag(currentVersion)) //nolint:errcheck // best-effort stderr
	fmt.Fprintf(stderr, "Update now? [y/N] ")                                                                                   //nolint:errcheck

	if !readYes(stdin) {
		return nil
	}

	return InstallRelease(ctx, cfg, InstallOptions{
		Tag:        result.Latest.TagName,
		TargetPath: "",
	})
}

func shouldSkipEnv() bool {
	if os.Getenv("OMNI_KUBECONFIG_SKIP_UPDATE_CHECK") == "1" {
		return true
	}
	if os.Getenv("CI") == "true" || os.Getenv("CI") == "1" {
		return true
	}
	return false
}

func isInteractive(stdin io.Reader) bool {
	if stdin != os.Stdin {
		return false
	}
	f, ok := stdin.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func readYes(r io.Reader) bool {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// InstallOptions controls binary installation.
type InstallOptions struct {
	Tag        string
	TargetPath string
	GoOS       string
	GoArch     string
}

// InstallLatest downloads and installs the latest stable release.
func InstallLatest(ctx context.Context, cfg Config, opts InstallOptions) error {
	latest, err := FetchLatestStable(ctx, cfg)
	if err != nil {
		return err
	}
	opts.Tag = latest.TagName
	return InstallRelease(ctx, cfg, opts)
}

// InstallRelease downloads tag to TargetPath or the running executable path.
func InstallRelease(ctx context.Context, cfg Config, opts InstallOptions) error {
	cfg = cfg.withDefaults()
	if opts.Tag == "" {
		return fmt.Errorf("release tag is required")
	}

	goos, goarch := opts.GoOS, opts.GoArch
	if goos == "" || goarch == "" {
		goos, goarch = CurrentPlatform()
	}

	assetName, err := AssetName(goos, goarch)
	if err != nil {
		return err
	}

	target, err := resolveInstallTarget(opts.TargetPath, goos)
	if err != nil {
		return err
	}

	binaryURL := ReleaseDownloadURL(cfg, opts.Tag, assetName)
	checksumURL := ReleaseDownloadURL(cfg, opts.Tag, "sha256sum.txt")

	checksums, err := downloadBytes(ctx, cfg, checksumURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	expected, err := ExpectedSHA256(checksums, assetName)
	if err != nil {
		return err
	}

	data, err := downloadBytes(ctx, cfg, binaryURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	if err := VerifySHA256(data, expected); err != nil {
		return fmt.Errorf("verify %s: %w", assetName, err)
	}

	if err := writeExecutable(target, data, goos); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Installed omni-kubeconfig %s to %s\n", normalizeTag(opts.Tag), target)
	return nil
}

func resolveInstallTarget(targetPath, goos string) (string, error) {
	if targetPath != "" {
		if goos == "windows" && !strings.HasSuffix(strings.ToLower(targetPath), ".exe") {
			return targetPath + ".exe", nil
		}
		return targetPath, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func downloadBytes(ctx context.Context, cfg Config, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cfg.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func writeExecutable(path string, data []byte, goos string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if goos == "windows" {
		return writeExecutableWindows(path, data)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".omni-kubeconfig-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func writeExecutableWindows(path string, data []byte) error {
	newPath := path + ".new"
	if err := os.WriteFile(newPath, data, 0o755); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(newPath)
		return err
	}
	if err := os.Rename(newPath, path); err != nil {
		return err
	}
	return nil
}

// DefaultInstallDir returns the default user install directory for goos.
func DefaultInstallDir(goos string) (string, error) {
	if goos == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", fmt.Errorf("LOCALAPPDATA is not set")
		}
		return filepath.Join(base, "Programs", "omni-kubeconfig"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}
