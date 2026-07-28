package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallShellScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh test runs on unix only")
	}

	script := filepath.Join("..", "..", "scripts", "install.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("stat install.sh: %v", err)
	}

	goos, goarch := CurrentPlatform()
	asset, err := AssetName(goos, goarch)
	if err != nil {
		t.Skip(err)
	}

	const tag = "v0.3.0"
	payload := []byte("#!/bin/sh\necho stub-0.3.0\n")
	sum, err := VerifySHA256Return(payload)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	releaseDir := filepath.Join(root, tag)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, asset), payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sums := fmt.Sprintf("%s  %s\n", sum, asset)
	if err := os.WriteFile(filepath.Join(releaseDir, "sha256sum.txt"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	latestJSON := fmt.Sprintf(`{"tag_name":"%s","prerelease":false}`, tag)
	if err := os.WriteFile(filepath.Join(root, "latest.json"), []byte(latestJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "repos", DefaultRepo, "releases"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "repos", DefaultRepo, "releases", "latest.json"), []byte(latestJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(root)))
	mux.HandleFunc("/repos/"+DefaultRepo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(root, "repos", DefaultRepo, "releases", "latest.json"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	installDir := filepath.Join(t.TempDir(), "bin")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"GITHUB_REPO="+DefaultRepo,
		"GITHUB_API_URL="+srv.URL,
		"GITHUB_DOWNLOAD_URL="+srv.URL,
		"VERSION="+tag,
		"INSTALL_DIR="+installDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}

	bin := filepath.Join(installDir, InstalledBinaryName(goos))
	st, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if st.Size() != int64(len(payload)) {
		t.Fatalf("binary size = %d want %d", st.Size(), len(payload))
	}

	// Bad checksum rejected
	badSums := filepath.Join(releaseDir, "sha256sum.txt")
	if err := os.WriteFile(badSums, []byte("deadbeef  "+asset+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"GITHUB_REPO="+DefaultRepo,
		"GITHUB_API_URL="+srv.URL,
		"GITHUB_DOWNLOAD_URL="+srv.URL,
		"VERSION="+tag,
		"INSTALL_DIR="+filepath.Join(t.TempDir(), "bad"),
	)
	if err := cmd.Run(); err == nil {
		t.Fatal("expected checksum failure")
	}

	// restore sums for upgrade test
	if err := os.WriteFile(badSums, []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}

	const tag2 = "v0.3.1"
	payload2 := []byte("#!/bin/sh\necho stub-0.3.1\n")
	sum2, err := VerifySHA256Return(payload2)
	if err != nil {
		t.Fatal(err)
	}
	releaseDir2 := filepath.Join(root, tag2)
	if err := os.MkdirAll(releaseDir2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir2, asset), payload2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir2, "sha256sum.txt"), []byte(fmt.Sprintf("%s  %s\n", sum2, asset)), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"GITHUB_REPO="+DefaultRepo,
		"GITHUB_API_URL="+srv.URL,
		"GITHUB_DOWNLOAD_URL="+srv.URL,
		"VERSION="+tag2,
		"INSTALL_DIR="+installDir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("upgrade install: %v\n%s", err, out)
	}
	runOut, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("run upgraded binary: %v", err)
	}
	if strings.TrimSpace(string(runOut)) != "stub-0.3.1" {
		t.Fatalf("upgrade output = %q", runOut)
	}
}

func TestInstallShellScriptResolveLatest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh test runs on unix only")
	}

	script := filepath.Join("..", "..", "scripts", "install.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("stat install.sh: %v", err)
	}

	goos, goarch := CurrentPlatform()
	asset, err := AssetName(goos, goarch)
	if err != nil {
		t.Skip(err)
	}

	const tag = "v0.3.2"
	payload := []byte("#!/bin/sh\necho stub-0.3.2\n")
	sum, err := VerifySHA256Return(payload)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	releaseDir := filepath.Join(root, tag)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, asset), payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sums := fmt.Sprintf("%s  %s\n", sum, asset)
	if err := os.WriteFile(filepath.Join(releaseDir, "sha256sum.txt"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(root)))
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead && r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Location", "/releases/tag/"+tag)
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/repos/"+DefaultRepo+"/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	installDir := filepath.Join(t.TempDir(), "bin")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"GITHUB_REPO="+DefaultRepo,
		"GITHUB_API_URL="+srv.URL,
		"GITHUB_DOWNLOAD_URL="+srv.URL,
		"GITHUB_LATEST_URL="+srv.URL+"/releases/latest",
		"INSTALL_DIR="+installDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh resolve latest: %v\n%s", err, out)
	}

	bin := filepath.Join(installDir, InstalledBinaryName(goos))
	runOut, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("run installed binary: %v", err)
	}
	if strings.TrimSpace(string(runOut)) != "stub-0.3.2" {
		t.Fatalf("output = %q", runOut)
	}
}
