package omni

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DefaultServiceAccountTTL matches omnictl kubeconfig --ttl.
const DefaultServiceAccountTTL = 365 * 24 * time.Hour

// DefaultServiceAccountGroups matches omnictl kubeconfig --groups.
var DefaultServiceAccountGroups = []string{constants.DefaultAccessGroup}

// KubeconfigOptions controls a single-cluster kubeconfig download.
type KubeconfigOptions struct {
	ClientOptions
	OutputPath       string
	Cluster          string
	ServiceAccount   bool
	User             string
	TTL              time.Duration
	Groups           []string
	Force            bool // overwrite existing file when not merging
	RenameOnConflict bool
	ActivateContext  bool
	GrantType        string
	BreakGlass       bool
	PrintExport      bool
	MergeExisting    bool
}

// Validate checks required flags for kubeconfig download.
func (opts KubeconfigOptions) Validate() error {
	if opts.Cluster == "" {
		return fmt.Errorf("--cluster is required")
	}
	if opts.ServiceAccount && opts.User == "" {
		return fmt.Errorf("--user is required when --service-account is set")
	}
	if opts.ServiceAccount && opts.TTL <= 0 {
		return fmt.Errorf("--ttl must be positive when --service-account is set")
	}
	return nil
}

// Kubeconfig downloads one cluster kubeconfig (OIDC admin or service-account token)
// and writes or merges it into OutputPath.
func Kubeconfig(opts KubeconfigOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	return WithClient(opts.ClientOptions, func(ctx context.Context, c *client.Client) error {
		return downloadKubeconfig(ctx, c, opts)
	})
}

func downloadKubeconfig(ctx context.Context, c *client.Client, opts KubeconfigOptions) error {
	names, err := listClusterNames(ctx, c, []string{opts.Cluster})
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("cluster not found: %q", opts.Cluster)
	}

	outputPath, err := prepareOutputPath(opts.OutputPath)
	if err != nil {
		return err
	}

	if err := ensureKubeconfigWritable(outputPath, opts); err != nil {
		return err
	}

	data, err := c.Management().WithCluster(opts.Cluster).Kubeconfig(ctx, kubeconfigAPIOptions(opts)...)
	if err != nil {
		if code, ok := status.FromError(err); ok && code.Code() == codes.NotFound {
			return fmt.Errorf("cluster %q not found", opts.Cluster)
		}
		return fmt.Errorf("download kubeconfig for %q: %w", opts.Cluster, err)
	}

	merger, err := mergerForSync(outputPath, opts.MergeExisting)
	if err != nil {
		return err
	}

	syncOpts := SyncOptions{
		RenameOnConflict: opts.RenameOnConflict,
		ActivateContext:  opts.ActivateContext,
	}
	if err := mergeKubeconfig(merger, data, syncOpts); err != nil {
		return fmt.Errorf("merge kubeconfig for %q: %w", opts.Cluster, err)
	}

	kind := "admin"
	if opts.ServiceAccount {
		kind = "service-account"
	}
	fmt.Fprintf(os.Stderr, "merged %s kubeconfig for %q\n", kind, opts.Cluster)

	return writeMergedKubeconfig(outputPath, merger, 1, opts.PrintExport)
}

func ensureKubeconfigWritable(path string, opts KubeconfigOptions) error {
	if opts.MergeExisting {
		return nil
	}

	_, err := os.Stat(path)
	if err == nil {
		if opts.Force {
			return nil
		}
		return fmt.Errorf("kubeconfig file already exists, use --force to overwrite: %q", path)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func kubeconfigAPIOptions(opts KubeconfigOptions) []management.KubeconfigOption {
	out := make([]management.KubeconfigOption, 0, 3)

	if opts.ServiceAccount {
		groups := opts.Groups
		if len(groups) == 0 {
			groups = DefaultServiceAccountGroups
		}
		out = append(out, management.WithServiceAccount(opts.TTL, opts.User, groups...))
	}

	out = appendGrantTypeOptions(out, opts.GrantType)

	if opts.BreakGlass {
		out = append(out, management.WithBreakGlassKubeconfig(true))
	}

	return out
}

func appendGrantTypeOptions(out []management.KubeconfigOption, grantType string) []management.KubeconfigOption {
	if grantType != "" {
		return append(out, management.WithGrantType(grantType))
	}
	return out
}
