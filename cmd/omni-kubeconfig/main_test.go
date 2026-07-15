package main

import (
	"path/filepath"
	"testing"
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
