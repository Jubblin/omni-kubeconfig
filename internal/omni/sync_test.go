package omni

import (
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
