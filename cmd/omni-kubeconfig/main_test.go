package main

import (
	"path/filepath"
	"testing"

	"github.com/Jubblin/omni-kubeconfig/internal/omni"
)

func testHome(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

func TestDefaultKubeconfigPath(t *testing.T) {
	home := testHome(t)

	got, err := defaultKubeconfigPath()
	if err != nil {
		t.Fatalf("defaultKubeconfigPath: %v", err)
	}

	want := filepath.Join(home, ".kube", "config")
	if got != want {
		t.Fatalf("defaultKubeconfigPath() = %q, want %q", got, want)
	}
}

func TestResolveOutputPathEmptyUsesDefault(t *testing.T) {
	home := testHome(t)

	got, err := resolveOutputPath("")
	if err != nil {
		t.Fatalf("resolveOutputPath: %v", err)
	}

	want := filepath.Join(home, ".kube", "config")
	if got != want {
		t.Fatalf("resolveOutputPath(\"\") = %q, want %q", got, want)
	}
}

func TestResolveOutputPathRelative(t *testing.T) {
	testHome(t)

	got, err := resolveOutputPath("custom/kubeconfig")
	if err != nil {
		t.Fatalf("resolveOutputPath: %v", err)
	}

	want, err := filepath.Abs("custom/kubeconfig")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveOutputPath(relative) = %q, want %q", got, want)
	}
}

func TestShouldPrintKubeconfigExport(t *testing.T) {
	home := testHome(t)
	defaultPath := filepath.Join(home, ".kube", "config")
	customPath := filepath.Join(home, ".kube", "custom")

	tests := []struct {
		name        string
		outputPath  string
		printExport bool
		want        bool
	}{
		{"default path", defaultPath, true, false},
		{"explicit default path", defaultPath, true, false},
		{"custom path", customPath, true, true},
		{"custom path export disabled", customPath, false, false},
		{"default path export disabled", defaultPath, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shouldPrintKubeconfigExport(tt.outputPath, tt.printExport)
			if err != nil {
				t.Fatalf("shouldPrintKubeconfigExport: %v", err)
			}
			if got != tt.want {
				t.Fatalf("shouldPrintKubeconfigExport(%q, %v) = %v, want %v",
					tt.outputPath, tt.printExport, got, tt.want)
			}
		})
	}
}

func TestSyncCommandDefaultOutputFlag(t *testing.T) {
	home := testHome(t)

	cmd := newRootCmd()
	syncCmd, _, err := cmd.Find([]string{"sync"})
	if err != nil {
		t.Fatalf("Find sync: %v", err)
	}

	flag := syncCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("output flag not found")
	}

	want := filepath.Join(home, ".kube", "config")
	if flag.DefValue != want {
		t.Fatalf("output flag default = %q, want %q", flag.DefValue, want)
	}
}

func TestSyncCommandActivateContextDefaultFalse(t *testing.T) {
	cmd := newRootCmd()
	syncCmd, _, err := cmd.Find([]string{"sync"})
	if err != nil {
		t.Fatalf("Find sync: %v", err)
	}

	flag := syncCmd.Flags().Lookup("activate-context")
	if flag == nil {
		t.Fatal("activate-context flag not found")
	}
	if flag.DefValue != "false" {
		t.Fatalf("activate-context default = %q, want false", flag.DefValue)
	}
}

func TestKubeconfigCommandFlags(t *testing.T) {
	home := testHome(t)

	cmd := newRootCmd()
	kubeCmd, _, err := cmd.Find([]string{"kubeconfig"})
	if err != nil {
		t.Fatalf("Find kubeconfig: %v", err)
	}

	cluster := kubeCmd.Flags().Lookup("cluster")
	if cluster == nil {
		t.Fatal("cluster flag not found")
	}

	output := kubeCmd.Flags().Lookup("output")
	if output == nil {
		t.Fatal("output flag not found")
	}
	wantOut := filepath.Join(home, ".kube", "config")
	if output.DefValue != wantOut {
		t.Fatalf("output default = %q, want %q", output.DefValue, wantOut)
	}

	ttl := kubeCmd.Flags().Lookup("ttl")
	if ttl == nil {
		t.Fatal("ttl flag not found")
	}
	if ttl.DefValue != omni.DefaultServiceAccountTTL.String() {
		t.Fatalf("ttl default = %q, want %q", ttl.DefValue, omni.DefaultServiceAccountTTL.String())
	}

	sa := kubeCmd.Flags().Lookup("service-account")
	if sa == nil || sa.DefValue != "false" {
		t.Fatalf("service-account default = %v", sa)
	}
}

func TestUpdateCommandRegistered(t *testing.T) {
	cmd := newRootCmd()
	updateCmd, _, err := cmd.Find([]string{"update"})
	if err != nil {
		t.Fatalf("Find update: %v", err)
	}
	if updateCmd.Use != "update" {
		t.Fatalf("update command = %q", updateCmd.Use)
	}
	if updateCmd.Flags().Lookup("check") == nil {
		t.Fatal("update --check flag missing")
	}
}

func TestMachineClassCloneCommandRegistered(t *testing.T) {
	cmd := newRootCmd()
	cloneCmd, _, err := cmd.Find([]string{"machineclass", "clone"})
	if err != nil {
		t.Fatalf("Find machineclass clone: %v", err)
	}
	if cloneCmd.Use != "clone <source> <destination>" {
		t.Fatalf("clone command = %q", cloneCmd.Use)
	}
	force := cloneCmd.Flags().Lookup("force")
	if force == nil {
		t.Fatal("machineclass clone --force flag missing")
	}
	if force.DefValue != "false" {
		t.Fatalf("force default = %q, want false", force.DefValue)
	}
}

func TestMachineClassListCommandRegistered(t *testing.T) {
	cmd := newRootCmd()
	listCmd, _, err := cmd.Find([]string{"machineclass", "list"})
	if err != nil {
		t.Fatalf("Find machineclass list: %v", err)
	}
	if listCmd.Use != "list" {
		t.Fatalf("list command = %q", listCmd.Use)
	}
}

func TestNormalizeVersion(t *testing.T) {
	if got := normalizeVersion("v0.3.0"); got != "0.3.0" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeVersion("0.3.0"); got != "0.3.0" {
		t.Fatalf("got %q", got)
	}
}
