package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	omnilibversion "github.com/siderolabs/omni/client/pkg/version"
	"github.com/spf13/cobra"

	"github.com/Jubblin/omni-kubeconfig/internal/omni"
	"github.com/Jubblin/omni-kubeconfig/internal/update"
	appversion "github.com/Jubblin/omni-kubeconfig/internal/version"
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
		noUpdateCheck         bool
		forceUpdateCheck      bool
	)

	root := &cobra.Command{
		Use:   "omni-kubeconfig",
		Short: "Download and merge kubeconfigs from a Sidero Omni server",
		Long: `Download admin kubeconfigs from a Sidero Omni server and merge them into one file for kubectl.

Commands:
  auth        Authenticate to Omni (SideroV1 PGP + browser login)
  sync        Download and merge OIDC admin kubeconfigs for clusters
  kubeconfig  Download one cluster kubeconfig (OIDC or Kubernetes service account)
  update      Install the latest release (self-update)

Install:
  curl -fsSL https://raw.githubusercontent.com/Jubblin/omni-kubeconfig/main/scripts/install.sh | bash

Global flags apply to all commands (see --help on each command for command-specific flags).`,
		Version: fmt.Sprintf("%s (Omni API %d)", appversion.String(), omniAPIVersion),
		Example: `  curl -fsSL https://raw.githubusercontent.com/Jubblin/omni-kubeconfig/main/scripts/install.sh | bash
  omni-kubeconfig auth
  omni-kubeconfig sync
  omni-kubeconfig update
  omni-kubeconfig sync --cluster prod --output ~/.kube/omni-prod
  omni-kubeconfig kubeconfig --service-account --cluster prod --user ci-deploy -o ./prod-ci.kubeconfig`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return maybeCheckForUpdates(cmd, noUpdateCheck, forceUpdateCheck)
		},
	}

	root.PersistentFlags().StringVar(&omniconfig, "omniconfig", "",
		"path to omniconfig (default: OMNICONFIG or ~/.talos/omni/config)")
	root.PersistentFlags().StringVar(&contextName, "context", "",
		"omniconfig context name (default: selected context)")
	root.PersistentFlags().BoolVar(&insecureSkipTLSVerify, "insecure-skip-tls-verify", false,
		"skip TLS verification for Omni API")
	root.PersistentFlags().StringVar(&sideroV1KeysDir, "siderov1-keys-dir", "",
		"path to SideroV1 auth keys (default: SIDEROV1_KEYS_DIR or ~/.talos/keys)")
	root.PersistentFlags().BoolVar(&noUpdateCheck, "no-update-check", false,
		"disable check for newer releases before running a command")
	root.PersistentFlags().BoolVar(&forceUpdateCheck, "check-updates", false,
		"force update check ignoring the 24h cache")

	clientOpts := func() omni.ClientOptions {
		return omni.ClientOptions{
			Omniconfig:            omniconfig,
			Context:               contextName,
			InsecureSkipTLSVerify: insecureSkipTLSVerify,
			SideroV1KeysDir:       sideroV1KeysDir,
		}
	}

	var (
		output           string
		clusters         []string
		renameOnConflict bool
		activateContext  bool
		grantType        string
		dryRun           bool
		printExport      bool
		mergeExisting    bool
	)

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Download kubeconfigs for all clusters and merge into one file",
		Long: `Connects to Omni, lists clusters (or only those named with --cluster), downloads each
admin kubeconfig, and merges them into a single file.

By default, an existing output file is loaded and new downloads are merged into it (incremental
update). Use --merge-existing=false to replace the file with only the clusters synced this run.

On name conflicts, incoming cluster/context/user entries overwrite existing ones by default.
Use --rename-on-conflict to keep both (incoming renamed to name-1, name-2, …).

By default, current-context is left unchanged when already set. An empty current-context
(fresh file or --merge-existing=false) is still activated on first merge. Use --activate-context
to always set current-context to the last cluster merged this run.

Existing output files are backed up to <path>.bak.<timestamp> before writing (both modes).
Merged kubeconfigs use kubectl oidc-login; run kubectl against the output file after sync.

Flags:
  -o, --output string         Path for the merged kubeconfig (default: ~/.kube/config)
  -c, --cluster strings       Sync only these cluster names (repeatable; default: all clusters)
      --merge-existing        Load existing output and merge new downloads (default: true)
      --rename-on-conflict    Rename conflicting entries instead of overwriting
      --activate-context      Set current-context to the last merged cluster; empty current-context still activates (default: false)
      --grant-type string     OIDC grant type in downloaded kubeconfigs: auto, authcode, authcode-keyboard
      --dry-run               List clusters that would be synced; do not download or write
      --print-export          Print "export KUBECONFIG=..." when -o is not the default path (default: true)

Global flags: --omniconfig, --context, --insecure-skip-tls-verify, --siderov1-keys-dir`,
		Example: `  omni-kubeconfig sync
  omni-kubeconfig sync --dry-run
  omni-kubeconfig sync -o ~/.kube/omni-prod -c prod -c staging
  omni-kubeconfig sync --rename-on-conflict
  omni-kubeconfig sync --activate-context
  omni-kubeconfig sync --merge-existing=false`,
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedOutput, err := resolveOutputPath(output)
			if err != nil {
				return err
			}

			printKubeconfigExport, err := shouldPrintKubeconfigExport(resolvedOutput, printExport)
			if err != nil {
				return err
			}

			return omni.Sync(omni.SyncOptions{
				ClientOptions:    clientOpts(),
				OutputPath:       resolvedOutput,
				Clusters:         clusters,
				RenameOnConflict: renameOnConflict,
				ActivateContext:  activateContext,
				GrantType:        grantType,
				DryRun:           dryRun,
				PrintExport:      printKubeconfigExport,
				MergeExisting:    mergeExisting,
			})
		},
	}

	defaultOutput, _ := defaultKubeconfigPath()
	syncCmd.Flags().StringVarP(&output, "output", "o", defaultOutput,
		"path for the merged kubeconfig file")
	syncCmd.Flags().StringSliceVarP(&clusters, "cluster", "c", nil,
		"only sync these Omni cluster names (repeat flag for multiple; default: all)")
	syncCmd.Flags().BoolVar(&mergeExisting, "merge-existing", true,
		"load existing output file and merge new downloads; false replaces file with clusters synced this run")
	syncCmd.Flags().BoolVar(&renameOnConflict, "rename-on-conflict", false,
		"on merge conflict, rename incoming cluster/context/user instead of overwriting")
	syncCmd.Flags().BoolVar(&activateContext, "activate-context", false,
		"set current-context to the last cluster merged this run; default preserves existing (activates when current-context is empty)")
	syncCmd.Flags().StringVar(&grantType, "grant-type", "auto",
		"OIDC grant type embedded in downloaded kubeconfigs (auto, authcode, authcode-keyboard)")
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"list clusters that would be synced without downloading or writing the output file")
	syncCmd.Flags().BoolVar(&printExport, "print-export", true,
		"print export KUBECONFIG=<path> after sync when -o is not ~/.kube/config")

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

	var (
		kubeOutput           string
		kubeCluster          string
		kubeServiceAccount   bool
		kubeUser             string
		kubeTTL              time.Duration
		kubeGroups           []string
		kubeForce            bool
		kubeRenameOnConflict bool
		kubeActivateContext  bool
		kubeGrantType        string
		kubeBreakGlass       bool
		kubePrintExport      bool
		kubeMergeExisting    bool
	)

	kubeCmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Download a kubeconfig for one Omni cluster",
		Long: `Download a kubeconfig for a single cluster via the Omni management API.

By default this requests an OIDC admin kubeconfig (same style as sync). With
--service-account, Omni mints a Kubernetes service-account token kubeconfig
(token-based; no kubelogin). This is cluster access — not an Omni API service
account (OMNI_SERVICE_ACCOUNT_KEY).

See: https://docs.siderolabs.com/omni/omni-cluster-setup/create-a-kubeconfig-for-a-service-account

Flags:
  -o, --output string         Path for the kubeconfig (default: ~/.kube/config)
  -c, --cluster string        Omni cluster name (required)
      --service-account       Request a service-account token kubeconfig
      --user string           Token subject (required with --service-account)
      --ttl duration          Service account token TTL (default: 8760h / 365d)
      --groups strings        Token groups (default: system:masters)
      --merge-existing        Merge into existing output (default: true)
      --force                 Overwrite existing file when --merge-existing=false
      --rename-on-conflict    Rename conflicting entries instead of overwriting
      --activate-context      Set current-context to this cluster
      --grant-type string     OIDC grant type for non-SA kubeconfigs
      --break-glass           Bypass Omni when enabled for the account
      --print-export          Print "export KUBECONFIG=..." when -o is not default

Global flags: --omniconfig, --context, --insecure-skip-tls-verify, --siderov1-keys-dir`,
		Example: `  omni-kubeconfig kubeconfig -c prod
  omni-kubeconfig kubeconfig --service-account -c prod --user ci-deploy -o ./prod-ci.kubeconfig
  omni-kubeconfig kubeconfig --service-account -c prod --user ci --ttl 720h --groups system:masters`,
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedOutput, err := resolveOutputPath(kubeOutput)
			if err != nil {
				return err
			}

			printKubeconfigExport, err := shouldPrintKubeconfigExport(resolvedOutput, kubePrintExport)
			if err != nil {
				return err
			}

			return omni.Kubeconfig(omni.KubeconfigOptions{
				ClientOptions:    clientOpts(),
				OutputPath:       resolvedOutput,
				Cluster:          kubeCluster,
				ServiceAccount:   kubeServiceAccount,
				User:             kubeUser,
				TTL:              kubeTTL,
				Groups:           kubeGroups,
				Force:            kubeForce,
				RenameOnConflict: kubeRenameOnConflict,
				ActivateContext:  kubeActivateContext,
				GrantType:        kubeGrantType,
				BreakGlass:       kubeBreakGlass,
				PrintExport:      printKubeconfigExport,
				MergeExisting:    kubeMergeExisting,
			})
		},
	}

	kubeCmd.Flags().StringVarP(&kubeOutput, "output", "o", defaultOutput,
		"path for the kubeconfig file")
	kubeCmd.Flags().StringVarP(&kubeCluster, "cluster", "c", "",
		"Omni cluster name (required)")
	kubeCmd.Flags().BoolVar(&kubeServiceAccount, "service-account", false,
		"create a Kubernetes service-account token kubeconfig instead of an OIDC user kubeconfig")
	kubeCmd.Flags().StringVar(&kubeUser, "user", "",
		"user (sub) for the service account token; required with --service-account")
	kubeCmd.Flags().DurationVar(&kubeTTL, "ttl", omni.DefaultServiceAccountTTL,
		"TTL for the service account token (only used with --service-account)")
	kubeCmd.Flags().StringSliceVar(&kubeGroups, "groups", append([]string(nil), omni.DefaultServiceAccountGroups...),
		"groups claim for the service account token (only used with --service-account)")
	kubeCmd.Flags().BoolVar(&kubeMergeExisting, "merge-existing", true,
		"load existing output file and merge; false replaces/writes only this cluster")
	kubeCmd.Flags().BoolVar(&kubeForce, "force", false,
		"overwrite existing output when --merge-existing=false")
	kubeCmd.Flags().BoolVar(&kubeRenameOnConflict, "rename-on-conflict", false,
		"on merge conflict, rename incoming cluster/context/user instead of overwriting")
	kubeCmd.Flags().BoolVar(&kubeActivateContext, "activate-context", false,
		"set current-context to this cluster; default preserves existing (activates when empty)")
	kubeCmd.Flags().StringVar(&kubeGrantType, "grant-type", "auto",
		"OIDC grant type embedded in non-service-account kubeconfigs (auto, authcode, authcode-keyboard)")
	kubeCmd.Flags().BoolVar(&kubeBreakGlass, "break-glass", false,
		"request a kubeconfig that bypasses Omni when enabled for the account")
	kubeCmd.Flags().BoolVar(&kubePrintExport, "print-export", true,
		"print export KUBECONFIG=<path> when -o is not ~/.kube/config")
	_ = kubeCmd.MarkFlagRequired("cluster")

	var (
		updateVersion string
		updateInstall string
		updateCheck   bool
	)

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Download and install a release of omni-kubeconfig",
		Long: `Install or upgrade omni-kubeconfig from GitHub releases.

By default installs the latest stable release over the running executable.
Use --install-dir to install to a directory instead (same layout as install.sh).

Examples:
  omni-kubeconfig update
  omni-kubeconfig update --check
  omni-kubeconfig update --version v0.3.0
  omni-kubeconfig update --install-dir ~/.local/bin`,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := update.Config{Repo: update.DefaultRepo}
			current := appversion.Version

			if updateCheck {
				res, err := update.Check(context.Background(), cfg, current, update.CheckOptions{
					SkipCheck:  noUpdateCheck,
					ForceCheck: true,
				})
				if err != nil {
					return err
				}
				if !res.Newer {
					fmt.Fprintf(os.Stderr, "omni-kubeconfig %s is up to date (latest %s)\n",
						normalizeVersion(current), res.Latest.Version)
					return nil
				}
				return fmt.Errorf("update available: %s (current %s)", res.Latest.Version, normalizeVersion(current))
			}

			opts := update.InstallOptions{TargetPath: updateInstall}
			if updateVersion != "" {
				tag := updateVersion
				if tag[0] != 'v' {
					tag = "v" + tag
				}
				opts.Tag = tag
				return update.InstallRelease(context.Background(), cfg, opts)
			}
			if updateInstall != "" {
				dir, err := filepath.Abs(updateInstall)
				if err != nil {
					return err
				}
				goos, _ := update.CurrentPlatform()
				opts.TargetPath = filepath.Join(dir, update.InstalledBinaryName(goos))
			}
			return update.InstallLatest(context.Background(), cfg, opts)
		},
	}
	updateCmd.Flags().StringVar(&updateVersion, "version", "",
		"install a specific release tag (e.g. v0.3.0) instead of latest stable")
	updateCmd.Flags().StringVar(&updateInstall, "install-dir", "",
		"install to this directory instead of replacing the running executable")
	updateCmd.Flags().BoolVar(&updateCheck, "check", false,
		"report whether a newer stable release exists (exit 1 if outdated)")

	root.AddCommand(authCmd)
	root.AddCommand(syncCmd)
	root.AddCommand(kubeCmd)
	root.AddCommand(updateCmd)

	return root
}

func maybeCheckForUpdates(cmd *cobra.Command, noUpdateCheck, forceUpdateCheck bool) error {
	if noUpdateCheck || cmd.Name() == "update" {
		return nil
	}
	if cmd.Root().Flags().Changed("version") {
		return nil
	}

	cfg := update.Config{Repo: update.DefaultRepo}
	return update.MaybePrompt(context.Background(), cfg, appversion.Version, update.CheckOptions{
		SkipCheck:  noUpdateCheck,
		ForceCheck: forceUpdateCheck,
		NoPrompt:   false,
	}, os.Stdin, os.Stderr)
}

func normalizeVersion(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

func defaultKubeconfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(home, ".kube", "config"))
}

func resolveOutputPath(path string) (string, error) {
	if path == "" {
		return defaultKubeconfigPath()
	}

	return filepath.Abs(path)
}

func shouldPrintKubeconfigExport(outputPath string, printExport bool) (bool, error) {
	if !printExport {
		return false, nil
	}

	defaultPath, err := defaultKubeconfigPath()
	if err != nil {
		return false, err
	}

	return outputPath != defaultPath, nil
}
