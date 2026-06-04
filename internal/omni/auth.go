package omni

import (
	"context"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/safe"
	pgpclient "github.com/siderolabs/go-api-signature/pkg/pgp/client"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
)

// Authenticate performs Omni SideroV1 login (same flow as omnictl on first API call).
// Opens the browser to register/confirm the PGP key when missing or expired.
// Use force to delete the existing key and re-authenticate.
func Authenticate(opts ClientOptions, force bool) error {
	omniCtx, err := ResolveContext(opts)
	if err != nil {
		return err
	}

	if omniCtx.ServiceAcct {
		return WithClient(opts, func(ctx context.Context, c *client.Client) error {
			return reportAuthSuccess(ctx, c, omniCtx.URL, "service account")
		})
	}

	if force {
		if err := deleteUserKey(opts, omniCtx.ContextName, omniCtx.Identity); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Removed existing PGP key for %q\n", omniCtx.Identity)
	}

	return WithClient(opts, func(ctx context.Context, c *client.Client) error {
		return reportAuthSuccess(ctx, c, omniCtx.URL, omniCtx.Identity)
	})
}

func reportAuthSuccess(ctx context.Context, c *client.Client, url, identity string) error {
	sysVersion, err := safe.StateGet[*system.SysVersion](ctx, c.Omni().State(),
		system.NewSysVersion(resources.EphemeralNamespace, system.SysVersionID).Metadata())
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Authenticated to %s as %s (server %s)\n",
		url, identity, sysVersion.TypedSpec().Value.BackendVersion)

	return nil
}

func deleteUserKey(opts ClientOptions, contextName, identity string) error {
	provider := newKeyProvider(opts.SideroV1KeysDir)

	err := provider.DeleteKey(contextName, identity)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete PGP key: %w", err)
	}

	return nil
}

func newKeyProvider(customKeysDir string) *pgpclient.KeyProvider {
	return pgpclient.NewKeyProviderWithFallback("omni/keys", config.CustomSideroV1KeysDirPath(customKeysDir), "", true)
}
