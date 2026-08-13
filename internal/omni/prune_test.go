package omni

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestPruneDestroyedOmniEntriesRemovesOIDCAndKeepsOthers(t *testing.T) {
	t.Parallel()

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["prod"] = &clientcmdapi.Cluster{Server: "https://prod.example"}
	cfg.Clusters["gone"] = &clientcmdapi.Cluster{Server: "https://gone.example"}
	cfg.Clusters["minikube"] = &clientcmdapi.Cluster{Server: "https://minikube"}
	cfg.AuthInfos["prod"] = oidcLoginAuth()
	cfg.AuthInfos["gone"] = oidcLoginAuth()
	cfg.AuthInfos["minikube"] = &clientcmdapi.AuthInfo{Token: "mk"}
	cfg.Contexts["prod"] = &clientcmdapi.Context{Cluster: "prod", AuthInfo: "prod"}
	cfg.Contexts["gone"] = &clientcmdapi.Context{Cluster: "gone", AuthInfo: "gone"}
	cfg.Contexts["minikube"] = &clientcmdapi.Context{Cluster: "minikube", AuthInfo: "minikube"}
	cfg.CurrentContext = "gone"

	removed := pruneDestroyedOmniEntries(cfg, filterSetFromNames([]string{"prod"}))
	if !slices.Equal(removed, []string{"gone"}) {
		t.Fatalf("removed = %v, want [gone]", removed)
	}
	if cfg.Contexts["gone"] != nil || cfg.Clusters["gone"] != nil || cfg.AuthInfos["gone"] != nil {
		t.Fatal("destroyed OIDC cluster entries should be removed")
	}
	if cfg.Contexts["prod"] == nil || cfg.Contexts["minikube"] == nil {
		t.Fatal("live Omni and non-Omni contexts should be kept")
	}
	if cfg.CurrentContext != "" {
		t.Fatalf("CurrentContext = %q, want empty after pruning active context", cfg.CurrentContext)
	}
}

func TestPruneDestroyedOmniEntriesRemovesServiceAccountNames(t *testing.T) {
	t.Parallel()

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["omni-prod"] = &clientcmdapi.Cluster{Server: "https://prod.example"}
	cfg.Clusters["omni-gone"] = &clientcmdapi.Cluster{Server: "https://gone.example"}
	cfg.AuthInfos["omni-prod-sa-ci"] = &clientcmdapi.AuthInfo{Token: "prod-token"}
	cfg.AuthInfos["omni-gone-sa-ci"] = &clientcmdapi.AuthInfo{Token: "gone-token"}
	cfg.Contexts["omni-prod-sa-ci"] = &clientcmdapi.Context{Cluster: "omni-prod", AuthInfo: "omni-prod-sa-ci"}
	cfg.Contexts["omni-gone-sa-ci"] = &clientcmdapi.Context{Cluster: "omni-gone", AuthInfo: "omni-gone-sa-ci"}

	removed := pruneDestroyedOmniEntries(cfg, filterSetFromNames([]string{"prod"}))
	if !slices.Equal(removed, []string{"omni-gone-sa-ci"}) {
		t.Fatalf("removed = %v, want [omni-gone-sa-ci]", removed)
	}
	if cfg.Contexts["omni-gone-sa-ci"] != nil || cfg.Clusters["omni-gone"] != nil {
		t.Fatal("destroyed SA cluster entries should be removed")
	}
	if cfg.Contexts["omni-prod-sa-ci"] == nil || cfg.Clusters["omni-prod"] == nil {
		t.Fatal("live SA cluster entries should be kept")
	}
}

func TestPruneKeepsOIDCWhenClusterIDHasOmniPrefix(t *testing.T) {
	t.Parallel()

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["omni-prod"] = &clientcmdapi.Cluster{Server: "https://prod.example"}
	cfg.AuthInfos["omni-prod"] = oidcLoginAuth()
	cfg.Contexts["omni-prod"] = &clientcmdapi.Context{Cluster: "omni-prod", AuthInfo: "omni-prod"}

	removed := pruneDestroyedOmniEntries(cfg, filterSetFromNames([]string{"omni-prod"}))
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none for live cluster named omni-prod", removed)
	}
}

func TestPruneDestroyedOmniEntriesNoOpWhenLive(t *testing.T) {
	t.Parallel()

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["prod"] = &clientcmdapi.Cluster{Server: "https://prod.example"}
	cfg.AuthInfos["prod"] = oidcLoginAuth()
	cfg.Contexts["prod"] = &clientcmdapi.Context{Cluster: "prod", AuthInfo: "prod"}

	removed := pruneDestroyedOmniEntries(cfg, filterSetFromNames([]string{"prod"}))
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", removed)
	}
	if cfg.Contexts["prod"] == nil {
		t.Fatal("live cluster should remain")
	}
}

func TestOmniIDFromSAName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		ok   bool
	}{
		{name: "omni-prod", id: "prod", ok: true},
		{name: "omni-prod-sa-ci", id: "prod", ok: true},
		{name: "prod", ok: false},
		{name: "omni-", ok: false},
		{name: "", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := omniIDFromSAName(tt.name)
			if ok != tt.ok || got != tt.id {
				t.Fatalf("omniIDFromSAName(%q) = %q, %v; want %q, %v", tt.name, got, ok, tt.id, tt.ok)
			}
		})
	}
}

func TestFilterClusterIDs(t *testing.T) {
	t.Parallel()

	live := []string{"alpha", "beta", "gamma"}

	got, err := filterClusterIDs(live, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, live) {
		t.Fatalf("no filter: got %v, want %v", got, live)
	}

	got, err = filterClusterIDs(live, []string{"beta"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"beta"}) {
		t.Fatalf("filter: got %v, want [beta]", got)
	}

	_, err = filterClusterIDs(live, []string{"missing"})
	if err == nil {
		t.Fatal("expected missing cluster error")
	}
}

func TestPrintSyncDryRunReportsPrune(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	cfg := clientcmdapi.NewConfig()
	cfg.Clusters["gone"] = &clientcmdapi.Cluster{Server: "https://gone.example"}
	cfg.AuthInfos["gone"] = oidcLoginAuth()
	cfg.Contexts["gone"] = &clientcmdapi.Context{Cluster: "gone", AuthInfo: "gone"}
	data, err := clientcmd.Write(*cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() error {
		return printSyncDryRun(SyncOptions{OutputPath: path, Prune: true}, []string{"prod"}, []string{"prod"})
	})
	if !strings.Contains(stderr, "would sync 1 cluster(s):") {
		t.Fatalf("stderr = %q, want sync header", stderr)
	}
	if !strings.Contains(stderr, "would prune 1 destroyed Omni cluster(s):") {
		t.Fatalf("stderr = %q, want prune header", stderr)
	}
	if !strings.Contains(stderr, "  gone\n") {
		t.Fatalf("stderr = %q, want pruned context name", stderr)
	}
}

func oidcLoginAuth() *clientcmdapi.AuthInfo {
	return &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    "kubectl",
			Args:       []string{"oidc-login", "get-token"},
		},
	}
}
