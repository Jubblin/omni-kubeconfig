package omni

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	managementapi "github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/constants"
)

func TestKubeconfigOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    KubeconfigOptions
		wantErr string
	}{
		{
			name:    "missing cluster",
			opts:    KubeconfigOptions{},
			wantErr: "--cluster is required",
		},
		{
			name: "service account without user",
			opts: KubeconfigOptions{
				Cluster:        "prod",
				ServiceAccount: true,
				TTL:            DefaultServiceAccountTTL,
			},
			wantErr: "--user is required when --service-account is set",
		},
		{
			name: "service account with non-positive ttl",
			opts: KubeconfigOptions{
				Cluster:        "prod",
				ServiceAccount: true,
				User:           "ci",
				TTL:            0,
			},
			wantErr: "--ttl must be positive when --service-account is set",
		},
		{
			name: "oidc admin ok",
			opts: KubeconfigOptions{
				Cluster: "prod",
			},
		},
		{
			name: "service account ok",
			opts: KubeconfigOptions{
				Cluster:        "prod",
				ServiceAccount: true,
				User:           "ci-deploy",
				TTL:            time.Hour,
				Groups:         []string{constants.DefaultAccessGroup},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("Validate() = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestKubeconfigAPIOptionsServiceAccount(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{
		Cluster:        "prod",
		ServiceAccount: true,
		User:           "ci",
		TTL:            2 * time.Hour,
		Groups:         []string{"system:masters", "developers"},
		GrantType:      "auto",
		BreakGlass:     true,
	}

	req := &managementapi.KubeconfigRequest{}
	for _, opt := range kubeconfigAPIOptions(opts) {
		opt(req)
	}

	if !req.ServiceAccount {
		t.Fatal("ServiceAccount not set")
	}
	if req.ServiceAccountUser != "ci" {
		t.Fatalf("ServiceAccountUser = %q, want ci", req.ServiceAccountUser)
	}
	if req.ServiceAccountTtl.AsDuration() != 2*time.Hour {
		t.Fatalf("ServiceAccountTtl = %v, want 2h", req.ServiceAccountTtl.AsDuration())
	}
	if len(req.ServiceAccountGroups) != 2 {
		t.Fatalf("ServiceAccountGroups = %#v", req.ServiceAccountGroups)
	}
	if req.GrantType != "auto" {
		t.Fatalf("GrantType = %q, want auto", req.GrantType)
	}
	if !req.BreakGlass {
		t.Fatal("BreakGlass not set")
	}
}

func TestKubeconfigAPIOptionsDefaultsGroups(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{
		Cluster:        "prod",
		ServiceAccount: true,
		User:           "ci",
		TTL:            time.Hour,
	}

	req := &managementapi.KubeconfigRequest{}
	for _, opt := range kubeconfigAPIOptions(opts) {
		opt(req)
	}

	if len(req.ServiceAccountGroups) != 1 || req.ServiceAccountGroups[0] != constants.DefaultAccessGroup {
		t.Fatalf("ServiceAccountGroups = %#v, want [%q]", req.ServiceAccountGroups, constants.DefaultAccessGroup)
	}
}

func TestKubeconfigAPIOptionsOmitsGrantTypeByDefault(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{
		Cluster: "prod",
	}

	req := &managementapi.KubeconfigRequest{}
	for _, opt := range kubeconfigAPIOptions(opts) {
		opt(req)
	}

	if req.GrantType != "" {
		t.Fatalf("GrantType = %q, want empty", req.GrantType)
	}
}

func TestKubeconfigAPIOptionsOIDCOnly(t *testing.T) {
	t.Parallel()

	opts := KubeconfigOptions{
		Cluster:   "prod",
		GrantType: "authcode",
	}

	req := &managementapi.KubeconfigRequest{}
	for _, opt := range kubeconfigAPIOptions(opts) {
		opt(req)
	}

	if req.ServiceAccount {
		t.Fatal("ServiceAccount should be false for OIDC admin kubeconfig")
	}
	if req.GrantType != "authcode" {
		t.Fatalf("GrantType = %q, want authcode", req.GrantType)
	}
}

func TestDefaultServiceAccountGroups(t *testing.T) {
	t.Parallel()

	if len(DefaultServiceAccountGroups) != 1 || DefaultServiceAccountGroups[0] != constants.DefaultAccessGroup {
		t.Fatalf("DefaultServiceAccountGroups = %#v, want [%q]", DefaultServiceAccountGroups, constants.DefaultAccessGroup)
	}
}

func TestEnsureKubeconfigWritable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kubeconfig")

	if err := ensureKubeconfigWritable(path, KubeconfigOptions{MergeExisting: false}); err != nil {
		t.Fatalf("missing file: %v", err)
	}

	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ensureKubeconfigWritable(path, KubeconfigOptions{MergeExisting: false})
	if err == nil {
		t.Fatal("expected exists error without --force")
	}

	if err := ensureKubeconfigWritable(path, KubeconfigOptions{MergeExisting: false, Force: true}); err != nil {
		t.Fatalf("force overwrite: %v", err)
	}

	if err := ensureKubeconfigWritable(path, KubeconfigOptions{MergeExisting: true}); err != nil {
		t.Fatalf("merge existing: %v", err)
	}
}
