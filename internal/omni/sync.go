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
	RenameOnConflict bool // when true, rename conflicting entries; when false (default), overwrite
	GrantType        string
	DryRun           bool
	PrintExport      bool
	MergeExisting    bool // when true, load existing output and merge; when false, replace with this run only
}

// Sync downloads kubeconfigs for all (or filtered) clusters and merges them.
func Sync(opts SyncOptions) error {
	return WithClient(opts.ClientOptions, func(ctx context.Context, c *client.Client) error {
		return syncClusters(ctx, c, opts)
	})
}

func syncClusters(ctx context.Context, c *client.Client, opts SyncOptions) error {
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
		return printDryRunClusters(names)
	}

	return syncClustersToFile(ctx, c, opts, names)
}

func printDryRunClusters(names []string) error {
	fmt.Fprintf(os.Stderr, "would sync %d cluster(s):\n", len(names))
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "  %s\n", name)
	}
	return nil
}

func syncClustersToFile(ctx context.Context, c *client.Client, opts SyncOptions, names []string) error {
	outputPath, err := prepareOutputPath(opts.OutputPath)
	if err != nil {
		return err
	}

	merger, err := mergerForSync(outputPath, opts.MergeExisting)
	if err != nil {
		return err
	}

	merged, err := fetchAndMergeKubeconfigs(ctx, c, names, opts, merger)
	if err != nil {
		return err
	}

	return writeMergedKubeconfig(outputPath, merger, merged, opts.PrintExport)
}

func prepareOutputPath(path string) (string, error) {
	outputPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	return outputPath, nil
}

func fetchAndMergeKubeconfigs(
	ctx context.Context,
	c *client.Client,
	names []string,
	opts SyncOptions,
	merger *clientcmdapi.Config,
) (int, error) {
	kubeOpts := []management.KubeconfigOption{
		management.WithGrantType(opts.GrantType),
	}

	var merged int
	var failures []string

	for _, name := range names {
		data, dlErr := c.Management().WithCluster(name).Kubeconfig(ctx, kubeOpts...)
		if dlErr != nil {
			logClusterDownloadWarning(name, dlErr)
			failures = append(failures, name)
			continue
		}

		if mergeErr := mergeKubeconfig(merger, data, opts.RenameOnConflict); mergeErr != nil {
			return 0, fmt.Errorf("merge kubeconfig for %q: %w", name, mergeErr)
		}

		merged++
		fmt.Fprintf(os.Stderr, "merged kubeconfig for %q\n", name)
	}

	if merged == 0 {
		return 0, fmt.Errorf("no kubeconfigs downloaded (%d failure(s))", len(failures))
	}

	return merged, nil
}

func logClusterDownloadWarning(name string, dlErr error) {
	if code, ok := status.FromError(dlErr); ok && code.Code() == codes.NotFound {
		fmt.Fprintf(os.Stderr, "[WARN] cluster %q not found, skipping\n", name)
		return
	}

	fmt.Fprintf(os.Stderr, "[WARN] cluster %q: %v\n", name, dlErr)
}

func writeMergedKubeconfig(outputPath string, merger *clientcmdapi.Config, merged int, printExport bool) error {
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
	if printExport {
		fmt.Printf("export KUBECONFIG=%s\n", outputPath)
	}

	return nil
}

func listClusterNames(ctx context.Context, c *client.Client, filter []string) ([]string, error) {
	list, err := safe.StateListAll[*omnires.Cluster](ctx, c.Omni().State())
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}

	names := matchingClusterNames(list, filterSetFromNames(filter))
	if err := validateFilterClusters(filter, names); err != nil {
		return nil, err
	}

	return names, nil
}

func filterSetFromNames(filter []string) map[string]struct{} {
	filterSet := make(map[string]struct{}, len(filter))
	for _, name := range filter {
		filterSet[name] = struct{}{}
	}
	return filterSet
}

func matchingClusterNames(list safe.List[*omnires.Cluster], filterSet map[string]struct{}) []string {
	var names []string

	for cluster := range list.All() {
		id := cluster.Metadata().ID()
		if !clusterMatchesFilter(id, filterSet) {
			continue
		}
		names = append(names, id)
	}

	return names
}

func clusterMatchesFilter(id string, filterSet map[string]struct{}) bool {
	if len(filterSet) == 0 {
		return true
	}

	_, ok := filterSet[id]
	return ok
}

func validateFilterClusters(filter, names []string) error {
	if len(filter) == 0 {
		return nil
	}

	var missing []string
	for _, name := range filter {
		if !slices.Contains(names, name) {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("cluster(s) not found: %s", strings.Join(missing, ", "))
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

func mergeKubeconfig(merger *clientcmdapi.Config, data []byte, renameOnConflict bool) error {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return err
	}

	m := (*kubeconfig.Merger)(merger)

	return m.Merge(cfg, kubeconfig.MergeOptions{
		ActivateContext: true,
		OutputWriter:    io.Discard,
		ConflictHandler: func(_ kubeconfig.ConfigComponent, _ string) (kubeconfig.ConflictDecision, error) {
			if renameOnConflict {
				return kubeconfig.RenameDecision, nil
			}
			return kubeconfig.OverwriteDecision, nil
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
