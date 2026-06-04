package omni

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
	"github.com/siderolabs/omni/client/pkg/version"
)

const endpointEnvVar = "OMNI_ENDPOINT"

// ClientOptions configures Omni API client bootstrap.
type ClientOptions struct {
	Omniconfig            string
	Context               string
	InsecureSkipTLSVerify bool
	SideroV1KeysDir       string
}

// WithClient connects to Omni and runs fn with an authenticated client.
// Authentication uses the same SideroV1 PGP + browser flow as omnictl (via gRPC interceptors).
func WithClient(opts ClientOptions, fn func(ctx context.Context, c *client.Client) error) error {
	if _, err := config.Init(opts.Omniconfig, false); err != nil {
		return fmt.Errorf("load omniconfig: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return withClientContext(ctx, opts, fn)
}

func withClientContext(ctx context.Context, opts ClientOptions, fn func(ctx context.Context, c *client.Client) error) error {
	omniCtx, err := ResolveContext(opts)
	if err != nil {
		return err
	}

	clientOpts := []client.Option{
		client.WithInsecureSkipTLSVerify(opts.InsecureSkipTLSVerify),
	}

	envKey, valueBase64 := serviceaccount.GetFromEnv()
	if envKey != "" {
		clientOpts = append(clientOpts, client.WithServiceAccount(valueBase64))
	} else {
		if conf, confErr := config.Current(); confErr == nil {
			if configCtx, ctxErr := conf.GetContext(opts.Context); ctxErr == nil && configCtx.Auth.Basic != "" { //nolint:staticcheck
				fmt.Fprintf(os.Stderr, "[WARN] basic auth is deprecated and has no effect\n")
			}
		}

		clientOpts = append(clientOpts,
			client.WithUserAccount(omniCtx.ContextName, omniCtx.Identity),
			client.WithCustomKeysDir(config.CustomSideroV1KeysDirPath(opts.SideroV1KeysDir)),
		)
	}

	loggerCfg := zap.NewDevelopmentConfig()
	loggerCfg.Development = false

	logger, err := loggerCfg.Build()
	if err != nil {
		return err
	}

	clientOpts = append(clientOpts, client.WithOmniClientOptions(omni.WithRetryLogger(logger)))

	c, err := client.New(omniCtx.URL, clientOpts...)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", omniCtx.URL, err)
	}
	defer c.Close() //nolint:errcheck

	// Bootstrap: triggers SideroV1 auth/re-auth (browser login) when the key is missing or expired.
	_, err = c.Omni().State().Get(ctx, system.NewSysVersion(system.SysVersionID).Metadata())
	if err != nil {
		return fmt.Errorf("authenticate with %s: %w (run: omni-kubeconfig auth)", omniCtx.URL, err)
	}

	if err = checkVersion(ctx, c.Omni().State()); err != nil {
		return err
	}

	return fn(ctx, c)
}

func checkVersion(ctx context.Context, st state.State) error {
	sysVersion, err := safe.StateGet[*system.SysVersion](ctx, st, system.NewSysVersion(system.SysVersionID).Metadata())
	if err != nil {
		return err
	}

	checkVersionWarning(sysVersion)

	if sysVersion.TypedSpec().Value.BackendApiVersion == 0 {
		return fmt.Errorf("server API does not support API versions; upgrade Omni server (client API %v, server %v)",
			version.API, sysVersion.TypedSpec().Value.BackendVersion)
	}

	if sysVersion.TypedSpec().Value.BackendApiVersion != version.API {
		return fmt.Errorf("client API version mismatch: backend %v, client %v",
			sysVersion.TypedSpec().Value.BackendApiVersion, version.API)
	}

	return nil
}

func checkVersionWarning(sysVersion *system.SysVersion) {
	backendVersion, err := semver.ParseTolerant(sysVersion.TypedSpec().Value.BackendVersion)
	if err != nil {
		return
	}

	clientVersion, err := semver.ParseTolerant(version.Tag)
	if err != nil {
		return
	}

	if clientVersion.Major != backendVersion.Major || clientVersion.Minor != backendVersion.Minor {
		fmt.Fprintf(os.Stderr, "[WARN] omni-kubeconfig version differs from backend: %q vs %q\n",
			clientVersion.String(), backendVersion.String())
	}
}
