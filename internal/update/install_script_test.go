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

type installReleaseFixture struct {
	tag     string
	payload []byte
}

type installMockServer struct {
	URL      string
	root     string
	asset    string
	releases map[string]installReleaseFixture
	latest   string
}

func installScriptPath(t *testing.T) string {
	t.Helper()
	script := filepath.Join("..", "..", "scripts", "install.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("stat install.sh: %v", err)
	}
	return script
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
}

func writeInstallReleaseFixtures(t *testing.T, root, asset string, releases []installReleaseFixture) map[string]installReleaseFixture {
	t.Helper()
	byTag := make(map[string]installReleaseFixture, len(releases))
	for _, rel := range releases {
		sum, err := VerifySHA256Return(rel.payload)
		if err != nil {
			t.Fatal(err)
		}
		releaseDir := filepath.Join(root, rel.tag)
		mustMkdirAll(t, releaseDir)
		mustWriteFile(t, filepath.Join(releaseDir, asset), rel.payload, 0o755)
		mustWriteFile(t, filepath.Join(releaseDir, "sha256sum.txt"), []byte(fmt.Sprintf("%s  %s\n", sum, asset)), 0o644)
		byTag[rel.tag] = rel
	}
	return byTag
}

func writeInstallAPIFixtures(t *testing.T, root, latestTag string, releases []installReleaseFixture) {
	t.Helper()
	apiDir := filepath.Join(root, "repos", DefaultRepo, "releases")
	mustMkdirAll(t, apiDir)
	mustWriteFile(t, filepath.Join(apiDir, "latest.json"),
		[]byte(fmt.Sprintf(`{"tag_name":"%s","prerelease":false}`, latestTag)), 0o644)

	var listJSON strings.Builder
	listJSON.WriteString("[")
	for i, rel := range releases {
		if i > 0 {
			listJSON.WriteString(",")
		}
		fmt.Fprintf(&listJSON, `{"tag_name":"%s","prerelease":false}`, rel.tag)
	}
	listJSON.WriteString("]")
	mustWriteFile(t, filepath.Join(apiDir, "list.json"), []byte(listJSON.String()), 0o644)
}

func registerInstallMockHandlers(mux *http.ServeMux, root, latestTag string, apiLatest404 bool) {
	mux.Handle("/", http.FileServer(http.Dir(root)))
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead && r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Location", "/releases/tag/"+latestTag)
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/repos/"+DefaultRepo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if apiLatest404 {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "repos", DefaultRepo, "releases", "latest.json"))
	})
	mux.HandleFunc("/repos/"+DefaultRepo+"/releases", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(root, "repos", DefaultRepo, "releases", "list.json"))
	})
}

func newInstallMockServer(t *testing.T, releases []installReleaseFixture, latestTag string, apiLatest404 bool) *installMockServer {
	t.Helper()

	goos, goarch := CurrentPlatform()
	asset, err := AssetName(goos, goarch)
	if err != nil {
		t.Skip(err)
	}

	root := t.TempDir()
	byTag := writeInstallReleaseFixtures(t, root, asset, releases)
	writeInstallAPIFixtures(t, root, latestTag, releases)

	mux := http.NewServeMux()
	registerInstallMockHandlers(mux, root, latestTag, apiLatest404)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &installMockServer{
		URL:      srv.URL,
		root:     root,
		asset:    asset,
		releases: byTag,
		latest:   latestTag,
	}
}

func runInstallScript(t *testing.T, mock *installMockServer, installDir string, extraEnv map[string]string) []byte {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("install.sh test runs on unix only")
	}

	cmd := exec.Command("bash", installScriptPath(t))
	env := append(os.Environ(),
		"GITHUB_REPO="+DefaultRepo,
		"GITHUB_API_URL="+mock.URL,
		"GITHUB_DOWNLOAD_URL="+mock.URL,
		"GITHUB_LATEST_URL="+mock.URL+"/releases/latest",
		"INSTALL_DIR="+installDir,
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	return out
}

func installedBinaryPath(t *testing.T, installDir string) string {
	t.Helper()
	goos, _ := CurrentPlatform()
	return filepath.Join(installDir, InstalledBinaryName(goos))
}

func assertInstalledBinary(t *testing.T, installDir, wantOutput string) {
	t.Helper()
	bin := installedBinaryPath(t, installDir)
	runOut, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("run installed binary: %v", err)
	}
	if got := strings.TrimSpace(string(runOut)); got != wantOutput {
		t.Fatalf("binary output = %q want %q", got, wantOutput)
	}
}

func TestInstallShellScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh test runs on unix only")
	}

	releases := []installReleaseFixture{
		{tag: "v0.2.0", payload: []byte("#!/bin/sh\necho stub-0.2.0\n")},
		{tag: "v0.3.1", payload: []byte("#!/bin/sh\necho stub-0.3.1\n")},
	}
	const latestTag = "v0.3.1"
	mock := newInstallMockServer(t, releases, latestTag, true)

	t.Run("latest", func(t *testing.T) {
		installDir := filepath.Join(t.TempDir(), "bin")
		runInstallScript(t, mock, installDir, nil)
		assertInstalledBinary(t, installDir, "stub-0.3.1")
	})

	t.Run("latest_specified", func(t *testing.T) {
		installDir := filepath.Join(t.TempDir(), "bin")
		runInstallScript(t, mock, installDir, map[string]string{"VERSION": latestTag})
		assertInstalledBinary(t, installDir, "stub-0.3.1")
	})

	t.Run("historical", func(t *testing.T) {
		installDir := filepath.Join(t.TempDir(), "bin")
		runInstallScript(t, mock, installDir, map[string]string{"VERSION": "v0.2.0"})
		assertInstalledBinary(t, installDir, "stub-0.2.0")
	})

	t.Run("upgrade", func(t *testing.T) {
		installDir := filepath.Join(t.TempDir(), "bin")
		runInstallScript(t, mock, installDir, map[string]string{"VERSION": "v0.2.0"})
		assertInstalledBinary(t, installDir, "stub-0.2.0")

		runInstallScript(t, mock, installDir, map[string]string{"VERSION": latestTag})
		assertInstalledBinary(t, installDir, "stub-0.3.1")
	})

	t.Run("checksum_rejected", func(t *testing.T) {
		sumsPath := filepath.Join(mock.root, latestTag, "sha256sum.txt")
		orig, err := os.ReadFile(sumsPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sumsPath, []byte("deadbeef  "+mock.asset+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.WriteFile(sumsPath, orig, 0o644)
		})

		cmd := exec.Command("bash", installScriptPath(t))
		cmd.Env = append(os.Environ(),
			"GITHUB_REPO="+DefaultRepo,
			"GITHUB_API_URL="+mock.URL,
			"GITHUB_DOWNLOAD_URL="+mock.URL,
			"VERSION="+latestTag,
			"INSTALL_DIR="+filepath.Join(t.TempDir(), "bad"),
		)
		if err := cmd.Run(); err == nil {
			t.Fatal("expected checksum failure")
		}
	})
}

func TestInstallShellScriptResolveLatestViaAPI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh test runs on unix only")
	}

	releases := []installReleaseFixture{
		{tag: "v0.2.0", payload: []byte("#!/bin/sh\necho stub-0.2.0\n")},
		{tag: "v0.3.2", payload: []byte("#!/bin/sh\necho stub-0.3.2\n")},
	}
	mock := newInstallMockServer(t, releases, "v0.3.2", false)

	installDir := filepath.Join(t.TempDir(), "bin")
	cmd := exec.Command("bash", installScriptPath(t))
	cmd.Env = append(os.Environ(),
		"GITHUB_REPO="+DefaultRepo,
		"GITHUB_API_URL="+mock.URL,
		"GITHUB_DOWNLOAD_URL="+mock.URL,
		"GITHUB_LATEST_URL="+mock.URL+"/releases/missing-latest",
		"INSTALL_DIR="+installDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh API latest fallback: %v\n%s", err, out)
	}
	assertInstalledBinary(t, installDir, "stub-0.3.2")
}
