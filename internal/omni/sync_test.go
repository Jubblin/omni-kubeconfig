package omni

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/go-kubeconfig"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const kubeconfigA = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://cluster-a.example:6443
  name: cluster-a
contexts:
- context:
    cluster: cluster-a
    user: user-a
  name: cluster-a
current-context: cluster-a
users:
- name: user-a
  user:
    token: token-a
`

const kubeconfigB = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://cluster-b.example:6443
  name: cluster-b
contexts:
- context:
    cluster: cluster-b
    user: user-b
  name: cluster-b
current-context: cluster-b
users:
- name: user-b
  user:
    token: token-b
`

func TestMergeKubeconfigCombinesClusters(t *testing.T) {
	merger := clientcmdapi.NewConfig()

	if err := mergeKubeconfig(merger, []byte(kubeconfigA), false); err != nil {
		t.Fatalf("merge A: %v", err)
	}
	if err := mergeKubeconfig(merger, []byte(kubeconfigB), false); err != nil {
		t.Fatalf("merge B: %v", err)
	}

	if len(merger.Clusters) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(merger.Clusters))
	}
	if len(merger.Contexts) != 2 {
		t.Fatalf("contexts: got %d, want 2", len(merger.Contexts))
	}
	if merger.Clusters["cluster-a"] == nil || merger.Clusters["cluster-b"] == nil {
		t.Fatal("expected both cluster entries")
	}
}

func TestLoadOrNewMergerEmptyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	cfg, err := loadOrNewMerger(path)
	if err != nil {
		t.Fatalf("loadOrNewMerger: %v", err)
	}
	if len(cfg.Clusters) != 0 {
		t.Fatalf("expected empty config, got %d clusters", len(cfg.Clusters))
	}
}

func TestMergerForSyncReplaceModeIgnoresExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte(kubeconfigA), 0o640); err != nil {
		t.Fatal(err)
	}

	cfg, err := mergerForSync(path, false)
	if err != nil {
		t.Fatalf("mergerForSync: %v", err)
	}
	if len(cfg.Clusters) != 0 {
		t.Fatalf("replace mode should start empty, got %d clusters", len(cfg.Clusters))
	}
}

func TestMergerForSyncMergeModeLoadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte(kubeconfigA), 0o640); err != nil {
		t.Fatal(err)
	}

	cfg, err := mergerForSync(path, true)
	if err != nil {
		t.Fatalf("mergerForSync: %v", err)
	}
	if cfg.Clusters["cluster-a"] == nil {
		t.Fatal("merge mode should load cluster-a from file")
	}
}

func TestMergerForSyncReplaceModeEmptyPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubeconfig")

	cfg, err := mergerForSync(path, false)
	if err != nil {
		t.Fatalf("mergerForSync: %v", err)
	}
	if len(cfg.Clusters) != 0 {
		t.Fatalf("expected empty config, got %d clusters", len(cfg.Clusters))
	}
}

func TestLoadOrNewMergerExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte(kubeconfigA), 0o640); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadOrNewMerger(path)
	if err != nil {
		t.Fatalf("loadOrNewMerger: %v", err)
	}
	if cfg.Clusters["cluster-a"] == nil {
		t.Fatal("expected cluster-a from file")
	}
}

func TestBackupIfExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte("data"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := backupIfExists(path); err != nil {
		t.Fatalf("backupIfExists: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("original file should be moved away")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "kubeconfig.bak.") {
		t.Fatalf("expected backup file, got %v", entries)
	}
}

func TestBackupIfExistsNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	if err := backupIfExists(path); err != nil {
		t.Fatalf("backupIfExists on missing: %v", err)
	}
}

func TestClusterMatchesFilter(t *testing.T) {
	filter := filterSetFromNames([]string{"prod", "staging"})

	if !clusterMatchesFilter("prod", filter) {
		t.Fatal("expected prod to match filter")
	}
	if clusterMatchesFilter("dev", filter) {
		t.Fatal("expected dev not to match filter")
	}
	if !clusterMatchesFilter("anything", nil) {
		t.Fatal("empty filter should match all clusters")
	}
	if !clusterMatchesFilter("anything", map[string]struct{}{}) {
		t.Fatal("empty filter set should match all clusters")
	}
}

func TestValidateFilterClusters(t *testing.T) {
	if err := validateFilterClusters(nil, []string{"a"}); err != nil {
		t.Fatalf("no filter: %v", err)
	}
	if err := validateFilterClusters([]string{}, []string{"a"}); err != nil {
		t.Fatalf("empty filter: %v", err)
	}
	if err := validateFilterClusters([]string{"a", "b"}, []string{"a", "b"}); err != nil {
		t.Fatalf("all present: %v", err)
	}

	err := validateFilterClusters([]string{"a", "missing"}, []string{"a"})
	if err == nil {
		t.Fatal("expected error for missing cluster")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v, want missing cluster name", err)
	}
}

func TestPrepareOutputPathCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "kubeconfig")

	got, err := prepareOutputPath(path)
	if err != nil {
		t.Fatalf("prepareOutputPath: %v", err)
	}

	want, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("prepareOutputPath() = %q, want %q", got, want)
	}

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("output directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected output parent directory to be created")
	}
}

func TestWriteMergedKubeconfigPrintExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "merged")

	merger := clientcmdapi.NewConfig()
	if err := mergeKubeconfig(merger, []byte(kubeconfigA), false); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() error {
		return writeMergedKubeconfig(path, merger, 1, true)
	})
	if !strings.Contains(stdout, "export KUBECONFIG=") {
		t.Fatalf("stdout = %q, want export line", stdout)
	}
	if !strings.Contains(stdout, path) {
		t.Fatalf("stdout = %q, want output path", stdout)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("output file: %v", err)
	}
}

func TestWriteMergedKubeconfigNoPrintExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "merged")

	merger := clientcmdapi.NewConfig()
	if err := mergeKubeconfig(merger, []byte(kubeconfigA), false); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() error {
		return writeMergedKubeconfig(path, merger, 1, false)
	})
	if strings.Contains(stdout, "export KUBECONFIG=") {
		t.Fatalf("stdout = %q, want no export line", stdout)
	}
}

func TestMergeKubeconfigForceOverwrite(t *testing.T) {
	conflictB := strings.Replace(kubeconfigB, "cluster-b", "cluster-a", -1)
	conflictB = strings.Replace(conflictB, "user-b", "user-a", -1)
	conflictB = strings.Replace(conflictB, "token-b", "token-conflict", -1)

	merger := clientcmdapi.NewConfig()
	if err := mergeKubeconfig(merger, []byte(kubeconfigA), false); err != nil {
		t.Fatal(err)
	}
	if err := mergeKubeconfig(merger, []byte(conflictB), true); err != nil {
		t.Fatal(err)
	}

	user := merger.AuthInfos["user-a"]
	if user == nil || user.Token != "token-conflict" {
		t.Fatalf("force merge should overwrite user-a token, got %#v", user)
	}
}

func TestMergeKubeconfigWithoutForceKeepsOriginalOnConflict(t *testing.T) {
	conflictB := strings.Replace(kubeconfigB, "cluster-b", "cluster-a", -1)
	conflictB = strings.Replace(conflictB, "user-b", "user-a", -1)
	conflictB = strings.Replace(conflictB, "token-b", "token-conflict", -1)

	merger := clientcmdapi.NewConfig()
	if err := mergeKubeconfig(merger, []byte(kubeconfigA), false); err != nil {
		t.Fatal(err)
	}
	if err := mergeKubeconfig(merger, []byte(conflictB), false); err != nil {
		t.Fatal(err)
	}

	user := merger.AuthInfos["user-a"]
	if user == nil || user.Token != "token-a" {
		t.Fatalf("without force, original user-a token should be kept, got %#v", user)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fnErr := fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if fnErr != nil {
		t.Fatalf("fn: %v", fnErr)
	}

	return buf.String()
}

func TestWriteMergedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "merged")

	merger := clientcmdapi.NewConfig()
	if err := mergeKubeconfig(merger, []byte(kubeconfigA), false); err != nil {
		t.Fatal(err)
	}
	if err := mergeKubeconfig(merger, []byte(kubeconfigB), false); err != nil {
		t.Fatal(err)
	}

	if err := (*kubeconfig.Merger)(merger).Write(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := clientcmd.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Clusters) != 2 {
		t.Fatalf("loaded clusters: %d", len(loaded.Clusters))
	}
}
