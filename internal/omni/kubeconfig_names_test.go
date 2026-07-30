package omni

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"k8s.io/client-go/tools/clientcmd"
)

const sampleServiceAccountKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://omni.example:443
  name: omni-mox-ci-deploy
contexts:
- context:
    cluster: omni-mox-ci-deploy
    namespace: default
    user: omni-mox-ci-deploy
  name: omni-mox-ci-deploy
current-context: omni-mox-ci-deploy
users:
- name: omni-mox-ci-deploy
  user:
    token: test-token
`

func TestNormalizeServiceAccountKubeconfig(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{
		Cluster:        "mox",
		User:           "ci-deploy",
		ServiceAccount: true,
	}

	out, err := normalizeServiceAccountKubeconfig([]byte(sampleServiceAccountKubeconfig), opts)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := clientcmd.Load(out)
	if err != nil {
		t.Fatal(err)
	}

	const wantCluster = "omni-mox"
	const wantUser = "omni-mox-sa-ci-deploy"
	if cfg.CurrentContext != wantUser {
		t.Fatalf("CurrentContext = %q, want %q", cfg.CurrentContext, wantUser)
	}
	if _, ok := cfg.Clusters[wantCluster]; !ok {
		t.Fatalf("cluster %q missing", wantCluster)
	}
	if _, ok := cfg.Contexts[wantUser]; !ok {
		t.Fatalf("context %q missing", wantUser)
	}
	if _, ok := cfg.AuthInfos[wantUser]; !ok {
		t.Fatalf("auth info %q missing", wantUser)
	}
	ctx := cfg.Contexts[wantUser]
	if ctx.Cluster != wantCluster || ctx.AuthInfo != wantUser {
		t.Fatalf("context refs = cluster %q auth %q, want %q / %q", ctx.Cluster, ctx.AuthInfo, wantCluster, wantUser)
	}
	if _, ok := cfg.Clusters["omni-mox-ci-deploy"]; ok {
		t.Fatal("old cluster name still present")
	}
}

func TestNormalizeServiceAccountKubeconfigWithUUIDUser(t *testing.T) {
	t.Parallel()

	const id = "018f5e12-3c4d-7890-a234-567890abcdef"
	raw := strings.ReplaceAll(sampleServiceAccountKubeconfig, "ci-deploy", id)

	opts := KubeconfigOptions{
		Cluster:        "mox",
		User:           id,
		ServiceAccount: true,
	}

	out, err := normalizeServiceAccountKubeconfig([]byte(raw), opts)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := clientcmd.Load(out)
	if err != nil {
		t.Fatal(err)
	}

	wantUser := "omni-mox-sa-" + id
	if cfg.AuthInfos[wantUser] == nil {
		t.Fatalf("auth info %q missing", wantUser)
	}
}

func TestNormalizeServiceAccountKubeconfigForceContextName(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{
		Cluster:          "mox",
		User:             "ci-deploy",
		ServiceAccount:   true,
		ForceContextName: "prod-ci",
	}

	out, err := normalizeServiceAccountKubeconfig([]byte(sampleServiceAccountKubeconfig), opts)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := clientcmd.Load(out)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.CurrentContext != "prod-ci" {
		t.Fatalf("CurrentContext = %q, want prod-ci", cfg.CurrentContext)
	}
	if cfg.Contexts["prod-ci"].AuthInfo != "omni-mox-sa-ci-deploy" {
		t.Fatalf("context auth = %q", cfg.Contexts["prod-ci"].AuthInfo)
	}
}

func TestOmniClusterPrefix(t *testing.T) {
	t.Parallel()

	got, ok := omniClusterPrefix("omni-mox-ci-deploy", "mox", "ci-deploy")
	if !ok || got != "omni-mox" {
		t.Fatalf("omniClusterPrefix() = (%q, %v), want (omni-mox, true)", got, ok)
	}
}

func TestNewUUIDV8String(t *testing.T) {
	t.Parallel()

	raw, err := newUUIDV8String()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Version() != 8 {
		t.Fatalf("uuid version = %d, want 8", parsed.Version())
	}
}

func TestEnsureServiceAccountUserGeneratesUUID(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{ServiceAccount: true}
	got, err := ensureServiceAccountUser(opts)
	if err != nil {
		t.Fatal(err)
	}
	if got.User == "" {
		t.Fatal("expected generated user")
	}
	if _, err := uuid.Parse(got.User); err != nil {
		t.Fatalf("generated user is not a uuid: %v", err)
	}
}
