package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jubblin/omni-kubeconfig/internal/omni"
	appversion "github.com/Jubblin/omni-kubeconfig/internal/version"
	omnilibversion "github.com/siderolabs/omni/client/pkg/version"
	"github.com/spf13/cobra"
)

const omniAPIVersion = 2

func init() {
	if err := appversion.Validate(); err != nil {
		panic(err)
	}

	// Required for Omni API client/server version checks (must match Omni server BackendApiVersion).
	omnilibversion.Name = "omni-kubeconfig"
	omnilibversion.API = omniAPIVersion
	omnilibversion.Tag = appversion.OmniTag()
	omnilibversion.SHA = appversion.Commit
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		omniconfig            string
		contextName           string
		insecureSkipTLSVerify bool
		sideroV1KeysDir       string
	)

	root := &cobra.Command{
		Use:   "omni-kubeconfig",
		Short: "Download and merge kubeconfigs from a Sidero Omni server",
		Long: `Download admin kubeconfigs from a Sidero Omni server and merge them into one file for kubectl.

Commands:
  auth   Authenticate to Omni (SideroV1 PGP + browser login)
  sync   Download and merge cluster kubeconfigs

Global flags apply to all commands (see --help on auth or sync for command-specific flags).`,
		Version: fmt.Sprintf("%s (Omni API %d)", appversion.String(), omniAPIVersion),
		Example: `  omni-kubeconfig auth
  omni-kubeconfig sync
  omni-kubeconfig sync --cluster prod --output ~/.kube/omni-prod`,
	}

	root.PersistentFlags().StringVar(&omniconfig, "omniconfig", "",
		"path to omniconfig (default: OMNICONFIG or ~/.talos/omni/config)")
	root.PersistentFlags().StringVar(&contextName, "context", "",
		"omniconfig context name (default: selected context)")
	root.PersistentFlags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", false,
		"skip TLS verification for Omni API")
	root.PersistentFlags().StringVar(&sideroV1KeysDir, "siderov1-keys-dir", "",
		"path to SideroV1 auth keys (default: SIDEROV1_KEYS_DIR or ~/.talos/keys)")

	clientOpts := func() omni.ClientOptions {
		return omni.ClientOptions{
			Omniconfig:            omniconfig,
			Context:               contextName,
			InsecureSkipTLSVerify: insecureSkipTLSVerify,
			SideroV1KeysDir:       sideroV1KeysDir,
		}
	}

	var (
		output        string
		clusters      []string
		force         bool
		grantType     string
		dryRun        bool
		printExport   bool
		mergeExisting bool
	)

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Download kubeconfigs for all clusters and merge into one file",
		Long: `Connects to Omni, lists clusters (or only those named with --cluster), downloads each
admin kubeconfig, and merges them into a single file.

By default, an existing output file is loaded and new downloads are merged into it (incremental
update). Use --merge-existing=false to replace the file with only the clusters synced this run.

Existing output files are backed up to <path>.bak.<timestamp> before writing (both modes).
Merged kubeconfigs use kubectl oidc-login; run kubectl against the output file after sync.

Flags:
  -o, --output string         Path for the merged kubeconfig (default: ~/.kube/omni-merged-config)
  -c, --cluster strings       Sync only these cluster names (repeatable; default: all clusters)
      --merge-existing        Load existing output and merge new downloads (default: true)
      --force                 Overwrite conflicting cluster/context/user entries on merge
      --grant-type string     OIDC grant type in downloaded kubeconfigs: auto, authcode, authcode-keyboard
      --dry-run               List clusters that would be synced; do not download or write
      --print-export          Print "export KUBECONFIG=..." to stdout on success (default: true)

Global flags: --omniconfig, --context, --insecure-skip-tls-verify, --siderov1-keys-dir`,
		Example: `  omni-kubeconfig sync
  omni-kubeconfig sync --dry-run
  omni-kubeconfig sync -o ~/.kube/omni-prod -c prod -c staging --force
  omni-kubeconfig sync --merge-existing=false`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if output == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				output = filepath.Join(home, ".kube", "omni-merged-config")
			}

			return omni.Sync(omni.SyncOptions{
				ClientOptions: clientOpts(),
				OutputPath:    output,
				Clusters:      clusters,
				Force:         force,
				GrantType:     grantType,
				DryRun:        dryRun,
				PrintExport:   printExport,
				MergeExisting: mergeExisting,
			})
		},
	}

	defaultOutput, _ := filepath.Abs(filepath.Join(mustHome(), ".kube", "omni-merged-config"))
	syncCmd.Flags().StringVarP(&output, "output", "o", defaultOutput,
		"path for the merged kubeconfig file")
	syncCmd.Flags().StringSliceVarP(&clusters, "cluster", "c", nil,
		"only sync these Omni cluster names (repeat flag for multiple; default: all)")
	syncCmd.Flags().BoolVar(&mergeExisting, "merge-existing", true,
		"load existing output file and merge new downloads; false replaces file with clusters synced this run")
	syncCmd.Flags().BoolVar(&force, "force", false,
		"on merge conflict, overwrite existing cluster/context/user instead of renaming")
	syncCmd.Flags().StringVar(&grantType, "grant-type", "auto",
		"OIDC grant type embedded in downloaded kubeconfigs (auto, authcode, authcode-keyboard)")
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"list clusters that would be synced without downloading or writing the output file")
	syncCmd.Flags().BoolVar(&printExport, "print-export", true,
		"print export KUBECONFIG=<path> to stdout after a successful sync")

	var authForce bool

	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate to Omni (SideroV1 PGP + browser login, same as omnictl)",
		Long: `Authenticate to the Omni API using the configured omniconfig context.

On first use or when the PGP key is expired, this opens a browser window to log in
(identity provider via Omni), registers a new key, and saves it under ~/.talos/keys.

Flags:
      --force   Delete the existing PGP key and force a new browser login

Global flags: --omniconfig, --context, --insecure-skip-tls-verify, --siderov1-keys-dir

Environment:
  BROWSER=echo   Print the login URL instead of opening a browser`,
		Example: `  omni-kubeconfig auth
  BROWSER=echo omni-kubeconfig auth
  omni-kubeconfig auth --force --context default`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return omni.Authenticate(clientOpts(), authForce)
		},
	}
	authCmd.Flags().BoolVar(&authForce, "force", false,
		"delete the existing PGP key for this context/identity and force a new browser login")

	root.AddCommand(authCmd)
	root.AddCommand(syncCmd)

	return root
}

func mustHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}
