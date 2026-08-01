package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAssetName(t *testing.T) {
	t.Parallel()

	got, err := AssetName("linux", "amd64")
	if err != nil || got != "omni-kubeconfig-linux-amd64" {
		t.Fatalf("linux/amd64 = %q err=%v", got, err)
	}

	_, err = AssetName("windows", "arm64")
	if err == nil {
		t.Fatal("expected windows/arm64 error")
	}
}

func TestExpectedSHA256(t *testing.T) {
	t.Parallel()

	data := []byte("abc123  omni-kubeconfig-linux-amd64\n")
	got, err := ExpectedSHA256(data, "omni-kubeconfig-linux-amd64")
	if err != nil || got != "abc123" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestVerifySHA256(t *testing.T) {
	t.Parallel()

	data := []byte("hello")
	sum := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if err := VerifySHA256(data, sum); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(data, "deadbeef"); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestIsDevVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{"0.3.0", false},
		{"0.3.0-dev", true},
		{"0.3.0-snapshot.1", true},
		{"0.3.0-3-gabc1234", true},
	}
	for _, tt := range tests {
		if got := IsDevVersion(tt.in); got != tt.want {
			t.Fatalf("IsDevVersion(%q) = %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestIsNewerStable(t *testing.T) {
	t.Parallel()

	ok, err := IsNewerStable("0.3.0", "0.3.1")
	if err != nil || !ok {
		t.Fatalf("0.3.1 > 0.3.0: ok=%v err=%v", ok, err)
	}
	ok, err = IsNewerStable("0.3.1", "0.3.0")
	if err != nil || ok {
		t.Fatalf("0.3.0 > 0.3.1: ok=%v err=%v", ok, err)
	}
	ok, err = IsNewerStable("0.3.1", "0.3.1")
	if err != nil || ok {
		t.Fatalf("0.3.1 == 0.3.1: ok=%v err=%v", ok, err)
	}
}

func TestFetchLatestStable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v0.3.1", Prerelease: false})
	}))
	defer srv.Close()

	cfg := Config{APIBaseURL: srv.URL, Repo: "Jubblin/omni-kubeconfig"}
	got, err := FetchLatestStable(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.TagName != "v0.3.1" || got.Version != "0.3.1" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckUsesCache(t *testing.T) {
	t.Parallel()

	apiCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v0.3.1", Prerelease: false})
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	cfg := Config{APIBaseURL: srv.URL, Repo: "Jubblin/omni-kubeconfig", CacheDir: cacheDir, CacheTTL: time.Hour}

	res, err := Check(context.Background(), cfg, "0.3.0", CheckOptions{})
	if err != nil || !res.Newer {
		t.Fatalf("first check: %+v err=%v", res, err)
	}
	if apiCalls != 1 {
		t.Fatalf("apiCalls = %d want 1", apiCalls)
	}

	res, err = Check(context.Background(), cfg, "0.3.0", CheckOptions{})
	if err != nil || !res.Newer {
		t.Fatalf("cached check: %+v err=%v", res, err)
	}
	if apiCalls != 1 {
		t.Fatalf("apiCalls after cache = %d want 1", apiCalls)
	}
}

func TestInstallRelease(t *testing.T) {
	t.Parallel()

	const asset = "omni-kubeconfig-linux-amd64"
	payload := []byte("#!/bin/sh\necho ok\n")
	expected, err := hashBytes(payload)
	if err != nil {
		t.Fatal(err)
	}
	sumLine := expected + "  " + asset + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/Jubblin/omni-kubeconfig/releases/download/v0.3.1/sha256sum.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sumLine))
	})
	mux.HandleFunc("/Jubblin/omni-kubeconfig/releases/download/v0.3.1/"+asset, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "omni-kubeconfig")
	cfg := Config{
		DownloadBaseURL: srv.URL,
		Repo:            "Jubblin/omni-kubeconfig",
	}

	err = InstallRelease(context.Background(), cfg, InstallOptions{
		Tag:        "v0.3.1",
		TargetPath: target,
		GoOS:       "linux",
		GoArch:     "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(target)
	if err != nil || st.Size() != int64(len(payload)) {
		t.Fatalf("installed binary: err=%v size=%d", err, st.Size())
	}
}

func TestMaybePromptNoWhenNonInteractive(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v9.9.9", Prerelease: false})
	}))
	defer srv.Close()

	cfg := Config{APIBaseURL: srv.URL, Repo: "Jubblin/omni-kubeconfig", CacheDir: t.TempDir()}
	err := MaybePrompt(context.Background(), cfg, "0.3.0", CheckOptions{NoPrompt: true}, strings.NewReader(""), os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMaybePromptSoftFailsOnCheckError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	var stderr strings.Builder
	cfg := Config{APIBaseURL: srv.URL, Repo: "Jubblin/omni-kubeconfig", CacheDir: t.TempDir()}
	err := MaybePrompt(context.Background(), cfg, "0.3.0", CheckOptions{ForceCheck: true}, strings.NewReader(""), &stderr)
	if err != nil {
		t.Fatalf("expected soft-fail nil, got %v", err)
	}
	if !strings.Contains(stderr.String(), "update check failed") {
		t.Fatalf("stderr = %q, want update check failed warning", stderr.String())
	}
}

func TestInstallLatestSkipsWhenCurrent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v0.3.4", Prerelease: false})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	var stderr strings.Builder
	cfg := Config{APIBaseURL: srv.URL, Repo: "Jubblin/omni-kubeconfig", CacheDir: t.TempDir()}
	err := InstallLatest(context.Background(), cfg, "0.3.4", InstallOptions{
		TargetPath: filepath.Join(t.TempDir(), "bin"),
	}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "already up to date") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestInstallLatestUpdatesWhenNewer(t *testing.T) {
	t.Parallel()

	const asset = "omni-kubeconfig-linux-amd64"
	payload := []byte("#!/bin/sh\necho newer\n")
	expected, err := hashBytes(payload)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/Jubblin/omni-kubeconfig/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v0.3.5", Prerelease: false})
	})
	mux.HandleFunc("/Jubblin/omni-kubeconfig/releases/download/v0.3.5/sha256sum.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(expected + "  " + asset + "\n"))
	})
	mux.HandleFunc("/Jubblin/omni-kubeconfig/releases/download/v0.3.5/"+asset, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "omni-kubeconfig")
	var stderr strings.Builder
	cfg := Config{
		APIBaseURL:      srv.URL,
		DownloadBaseURL: srv.URL,
		Repo:            "Jubblin/omni-kubeconfig",
		CacheDir:        t.TempDir(),
	}
	err = InstallLatest(context.Background(), cfg, "0.3.4", InstallOptions{
		TargetPath: target,
		GoOS:       "linux",
		GoArch:     "amd64",
	}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(target)
	if err != nil || st.Size() != int64(len(payload)) {
		t.Fatalf("installed binary: err=%v size=%d", err, st.Size())
	}
}

func TestReadYes(t *testing.T) {
	t.Parallel()

	if !readYes(strings.NewReader("yes\n")) {
		t.Fatal("expected yes")
	}
	if readYes(strings.NewReader("no\n")) {
		t.Fatal("expected no")
	}
}

func hashBytes(data []byte) (string, error) {
	return VerifySHA256Return(data)
}
