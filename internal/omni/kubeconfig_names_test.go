package omni

import (
	"testing"

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

	const want = "omni-mox"
	if cfg.CurrentContext != want {
		t.Fatalf("CurrentContext = %q, want %q", cfg.CurrentContext, want)
	}
	if _, ok := cfg.Contexts[want]; !ok {
		t.Fatalf("context %q missing", want)
	}
	if _, ok := cfg.Clusters[want]; !ok {
		t.Fatalf("cluster %q missing", want)
	}
	if _, ok := cfg.AuthInfos[want]; !ok {
		t.Fatalf("auth info %q missing", want)
	}
	if cfg.Contexts[want].Cluster != want || cfg.Contexts[want].AuthInfo != want {
		t.Fatalf("context refs = %#v, want %q", cfg.Contexts[want], want)
	}
	if _, ok := cfg.Contexts["omni-mox-ci-deploy"]; ok {
		t.Fatal("old context name still present")
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
}

func TestDefaultServiceAccountContextName(t *testing.T) {
	t.Parallel()

	got, ok := defaultServiceAccountContextName("omni-mox-ci-deploy", "mox", "ci-deploy")
	if !ok || got != "omni-mox" {
		t.Fatalf("defaultServiceAccountContextName() = (%q, %v), want (omni-mox, true)", got, ok)
	}

	if _, ok := defaultServiceAccountContextName("omni-mox", "mox", "ci-deploy"); ok {
		t.Fatal("expected false for name without user suffix")
	}
}
