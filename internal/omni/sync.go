package omni

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-kubeconfig"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// SyncOptions controls kubeconfig download and merge.
type SyncOptions struct {
	ClientOptions
	OutputPath    string
	Clusters      []string
	Force         bool
	GrantType     string
	DryRun        bool
	PrintExport   bool
	MergeExisting bool // when true, load existing output and merge; when false, replace with this run only
}

// Sync downloads kubeconfigs for all (or filtered) clusters and merges them.
func Sync(opts SyncOptions) error {
	var merged int

	return WithClient(opts.ClientOptions, func(ctx context.Context, c *client.Client) error {
		names, err := listClusterNames(ctx, c, opts.Clusters)
		if err != nil {
			return err
		}

		if len(names) == 0 {
			fmt.Fprintln(os.Stderr, "no clusters found")
			return nil
		}

		slices.Sort(names)

		if opts.DryRun {
			fmt.Fprintf(os.Stderr, "would sync %d cluster(s):\n", len(names))
			for _, name := range names {
				fmt.Fprintf(os.Stderr, "  %s\n", name)
			}
			return nil
		}

		outputPath, err := filepath.Abs(opts.OutputPath)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}

		merger, err := mergerForSync(outputPath, opts.MergeExisting)
		if err != nil {
			return err
		}

		kubeOpts := []management.KubeconfigOption{
			management.WithGrantType(opts.GrantType),
		}

		var failures []string

		for _, name := range names {
			data, dlErr := c.Management().WithCluster(name).Kubeconfig(ctx, kubeOpts...)
			if dlErr != nil {
				if code, ok := status.FromError(dlErr); ok && code.Code() == codes.NotFound {
					fmt.Fprintf(os.Stderr, "[WARN] cluster %q not found, skipping\n", name)
				} else {
					fmt.Fprintf(os.Stderr, "[WARN] cluster %q: %v\n", name, dlErr)
				}
				failures = append(failures, name)
				continue
			}

			if mergeErr := mergeKubeconfig(merger, data, opts.Force); mergeErr != nil {
				return fmt.Errorf("merge kubeconfig for %q: %w", name, mergeErr)
			}

			merged++
			fmt.Fprintf(os.Stderr, "merged kubeconfig for %q\n", name)
		}

		if merged == 0 {
			return fmt.Errorf("no kubeconfigs downloaded (%d failure(s))", len(failures))
		}

		if err := backupIfExists(outputPath); err != nil {
			return err
		}

		if err := (*kubeconfig.Merger)(merger).Write(outputPath); err != nil {
			return fmt.Errorf("write %q: %w", outputPath, err)
		}

		if err := os.Chmod(outputPath, 0o640); err != nil {
			return fmt.Errorf("chmod %q: %w", outputPath, err)
		}

		fmt.Fprintf(os.Stderr, "Merged %d cluster(s) into %s\n", merged, outputPath)
		if opts.PrintExport {
			fmt.Printf("export KUBECONFIG=%s\n", outputPath)
		}

		return nil
	})
}

func listClusterNames(ctx context.Context, c *client.Client, filter []string) ([]string, error) {
	list, err := safe.StateListAll[*omnires.Cluster](ctx, c.Omni().State())
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}

	filterSet := make(map[string]struct{}, len(filter))
	for _, name := range filter {
		filterSet[name] = struct{}{}
	}

	var names []string

	for cluster := range list.All() {
		id := cluster.Metadata().ID()
		if len(filterSet) > 0 {
			if _, ok := filterSet[id]; !ok {
				continue
			}
		}
		names = append(names, id)
	}

	if len(filter) > 0 {
		var missing []string
		for _, name := range filter {
			if !slices.Contains(names, name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("cluster(s) not found: %s", strings.Join(missing, ", "))
		}
	}

	return names, nil
}

func mergerForSync(path string, mergeExisting bool) (*clientcmdapi.Config, error) {
	if !mergeExisting {
		return clientcmdapi.NewConfig(), nil
	}

	return loadOrNewMerger(path)
}

func loadOrNewMerger(path string) (*clientcmdapi.Config, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return clientcmdapi.NewConfig(), nil
		}
		return nil, err
	}

	loaded, err := kubeconfig.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load existing %q: %w", path, err)
	}

	return (*clientcmdapi.Config)(loaded), nil
}

func mergeKubeconfig(merger *clientcmdapi.Config, data []byte, force bool) error {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return err
	}

	m := (*kubeconfig.Merger)(merger)

	return m.Merge(cfg, kubeconfig.MergeOptions{
		ActivateContext: true,
		OutputWriter:    io.Discard,
		ConflictHandler: func(_ kubeconfig.ConfigComponent, _ string) (kubeconfig.ConflictDecision, error) {
			if force {
				return kubeconfig.OverwriteDecision, nil
			}
			return kubeconfig.RenameDecision, nil
		},
	})
}

func backupIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	backup := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
	if err := os.Rename(path, backup); err != nil {
		return fmt.Errorf("backup %q to %q: %w", path, backup, err)
	}

	fmt.Fprintf(os.Stderr, "backed up existing config to %s\n", backup)
	return nil
}
