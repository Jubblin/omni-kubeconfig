package omni

import (
	"reflect"
	"slices"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/omni/client/api/omni/specs"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestValidateCloneMachineClassNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      string
		destination string
		wantErr     bool
	}{
		{name: "ok", source: "a", destination: "b"},
		{name: "empty_source", source: "", destination: "b", wantErr: true},
		{name: "empty_destination", source: "a", destination: "", wantErr: true},
		{name: "same_names", source: "workers", destination: "workers", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateCloneMachineClassNames(tt.source, tt.destination)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCloneMachineClassResource(t *testing.T) {
	t.Parallel()

	src := omnires.NewMachineClass("workers")
	src.Metadata().Labels().Set("env", "prod")
	src.Metadata().Labels().Set("team", "platform")
	src.TypedSpec().Value.MatchLabels = []string{
		"omni.sidero.dev/arch = amd64",
		"omni.sidero.dev/cores > 2",
	}
	src.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{
		ProviderId: "aws",
		KernelArgs: []string{"console=ttyS0"},
	}

	dest := cloneMachineClassResource(src, "workers-copy")

	if dest.Metadata().ID() != "workers-copy" {
		t.Fatalf("dest id = %q, want workers-copy", dest.Metadata().ID())
	}
	if src.Metadata().ID() != "workers" {
		t.Fatalf("src id mutated to %q", src.Metadata().ID())
	}

	if !reflect.DeepEqual(dest.TypedSpec().Value.MatchLabels, src.TypedSpec().Value.MatchLabels) {
		t.Fatalf("MatchLabels = %#v, want %#v", dest.TypedSpec().Value.MatchLabels, src.TypedSpec().Value.MatchLabels)
	}
	if dest.TypedSpec().Value.AutoProvision == nil || dest.TypedSpec().Value.AutoProvision.ProviderId != "aws" {
		t.Fatalf("AutoProvision = %#v", dest.TypedSpec().Value.AutoProvision)
	}

	// Mutating dest must not affect src (CloneVT deep copy).
	dest.TypedSpec().Value.MatchLabels[0] = "mutated"
	dest.TypedSpec().Value.AutoProvision.ProviderId = "mutated"
	if src.TypedSpec().Value.MatchLabels[0] != "omni.sidero.dev/arch = amd64" {
		t.Fatal("src MatchLabels mutated via dest")
	}
	if src.TypedSpec().Value.AutoProvision.ProviderId != "aws" {
		t.Fatal("src AutoProvision mutated via dest")
	}

	for _, key := range []string{"env", "team"} {
		got, ok := dest.Metadata().Labels().Get(key)
		want, _ := src.Metadata().Labels().Get(key)
		if !ok || got != want {
			t.Fatalf("label %q = %q ok=%v, want %q", key, got, ok, want)
		}
	}
}

func TestApplyMachineClassCloneReplacesLabels(t *testing.T) {
	t.Parallel()

	src := omnires.NewMachineClass("src")
	src.Metadata().Labels().Set("keep", "yes")
	src.TypedSpec().Value.MatchLabels = []string{"a = b"}

	dest := omnires.NewMachineClass("dst")
	dest.Metadata().Labels().Set("stale", "old")
	dest.TypedSpec().Value.MatchLabels = []string{"old = label"}

	applyMachineClassClone(dest, src)

	if _, ok := dest.Metadata().Labels().Get("stale"); ok {
		t.Fatal("stale label should have been removed")
	}
	if got, ok := dest.Metadata().Labels().Get("keep"); !ok || got != "yes" {
		t.Fatalf("keep label = %q ok=%v", got, ok)
	}
	if !reflect.DeepEqual(dest.TypedSpec().Value.MatchLabels, []string{"a = b"}) {
		t.Fatalf("MatchLabels = %#v", dest.TypedSpec().Value.MatchLabels)
	}
}

func TestMachineClassNames(t *testing.T) {
	t.Parallel()

	got := machineClassNames(testMachineClassList("workers", "control-plane", "gpu"))
	want := []string{"workers", "control-plane", "gpu"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("machineClassNames = %#v, want %#v", got, want)
	}

	sorted := append([]string(nil), got...)
	slices.Sort(sorted)
	if !reflect.DeepEqual(sorted, []string{"control-plane", "gpu", "workers"}) {
		t.Fatalf("sorted = %#v", sorted)
	}

	empty := machineClassNames(testMachineClassList())
	if len(empty) != 0 {
		t.Fatalf("empty list = %#v", empty)
	}
}

func testMachineClassList(ids ...string) safe.List[*omnires.MachineClass] {
	items := make([]resource.Resource, len(ids))
	for i, id := range ids {
		items[i] = omnires.NewMachineClass(id)
	}
	return safe.NewList[*omnires.MachineClass](resource.List{Items: items})
}
