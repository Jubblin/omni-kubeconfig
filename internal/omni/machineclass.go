package omni

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/omni/client/pkg/client"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// CloneMachineClassOptions controls MachineClass cloning.
type CloneMachineClassOptions struct {
	ClientOptions
	Source      string
	Destination string
	Force       bool
}

// ListMachineClassesOptions controls MachineClass listing.
type ListMachineClassesOptions struct {
	ClientOptions
}

// CloneMachineClass copies an Omni MachineClass to a new ID.
// TypedSpec and user metadata labels are copied. Create-only unless Force is set.
func CloneMachineClass(opts CloneMachineClassOptions) error {
	if err := validateCloneMachineClassNames(opts.Source, opts.Destination); err != nil {
		return err
	}

	return WithClient(opts.ClientOptions, func(ctx context.Context, c *client.Client) error {
		return cloneMachineClass(ctx, c.Omni().State(), opts)
	})
}

// ListMachineClasses prints MachineClass IDs (sorted) one per line to stdout.
func ListMachineClasses(opts ListMachineClassesOptions) error {
	return WithClient(opts.ClientOptions, func(ctx context.Context, c *client.Client) error {
		return listMachineClasses(ctx, c.Omni().State(), os.Stdout)
	})
}

func validateCloneMachineClassNames(source, destination string) error {
	if source == "" || destination == "" {
		return fmt.Errorf("source and destination machine class names are required")
	}
	if source == destination {
		return fmt.Errorf("source and destination machine class names must differ")
	}
	return nil
}

func listMachineClasses(ctx context.Context, st state.State, out io.Writer) error {
	list, err := safe.StateListAll[*omnires.MachineClass](ctx, st)
	if err != nil {
		return fmt.Errorf("list machine classes: %w", err)
	}

	names := machineClassNames(list)
	slices.Sort(names)

	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "no machine classes found")
		return nil
	}

	for _, name := range names {
		fmt.Fprintln(out, name)
	}
	return nil
}

func machineClassNames(list safe.List[*omnires.MachineClass]) []string {
	var names []string
	for mc := range list.All() {
		names = append(names, mc.Metadata().ID())
	}
	return names
}

func cloneMachineClass(ctx context.Context, st state.State, opts CloneMachineClassOptions) error {
	src, err := safe.StateGet[*omnires.MachineClass](ctx, st, omnires.NewMachineClass(opts.Source).Metadata())
	if err != nil {
		return fmt.Errorf("get machine class %q: %w", opts.Source, err)
	}

	dest := cloneMachineClassResource(src, opts.Destination)

	existing, err := safe.StateGet[*omnires.MachineClass](ctx, st, dest.Metadata())
	if err == nil {
		if !opts.Force {
			return fmt.Errorf("machine class %q already exists (use --force to overwrite)", opts.Destination)
		}
		return forceUpdateMachineClass(ctx, st, existing, src, opts)
	}
	if !state.IsNotFoundError(err) {
		return fmt.Errorf("get machine class %q: %w", opts.Destination, err)
	}

	if err := st.Create(ctx, dest); err != nil {
		return fmt.Errorf("create machine class %q: %w", opts.Destination, err)
	}

	fmt.Fprintf(os.Stderr, "cloned machine class %s → %s\n", opts.Source, opts.Destination)
	return nil
}

func forceUpdateMachineClass(
	ctx context.Context,
	st state.State,
	existing, src *omnires.MachineClass,
	opts CloneMachineClassOptions,
) error {
	_, err := safe.StateUpdateWithConflicts(ctx, st, existing.Metadata(), func(res *omnires.MachineClass) error {
		applyMachineClassClone(res, src)
		return nil
	})
	if err != nil {
		return fmt.Errorf("update machine class %q: %w", opts.Destination, err)
	}

	fmt.Fprintf(os.Stderr, "cloned machine class %s → %s (overwrote existing)\n", opts.Source, opts.Destination)
	return nil
}

// cloneMachineClassResource builds a new MachineClass with destID, copying TypedSpec and labels from src.
func cloneMachineClassResource(src *omnires.MachineClass, destID string) *omnires.MachineClass {
	dest := omnires.NewMachineClass(destID)
	applyMachineClassClone(dest, src)
	return dest
}

func applyMachineClassClone(dest, src *omnires.MachineClass) {
	dest.TypedSpec().Value = src.TypedSpec().Value.CloneVT()
	replaceMachineClassLabels(dest, src)
}

func replaceMachineClassLabels(dest, src *omnires.MachineClass) {
	for _, key := range dest.Metadata().Labels().Keys() {
		dest.Metadata().Labels().Delete(key)
	}
	for key, value := range src.Metadata().Labels().Raw() {
		dest.Metadata().Labels().Set(key, value)
	}
}
