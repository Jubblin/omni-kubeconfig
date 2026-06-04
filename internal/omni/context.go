package omni

import (
	"fmt"
	"os"

	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/siderolabs/omni/client/pkg/omnictl/config"
)

// OmniContext holds resolved connection settings from omniconfig.
type OmniContext struct {
	URL         string
	ContextName string
	Identity    string
	ServiceAcct bool
}

// ResolveContext loads omniconfig and returns connection settings.
func ResolveContext(opts ClientOptions) (OmniContext, error) {
	conf, err := config.Init(opts.Omniconfig, false)
	if err != nil {
		return OmniContext{}, fmt.Errorf("load omniconfig: %w", err)
	}

	envKey, _ := serviceaccount.GetFromEnv()
	if envKey != "" {
		url := os.Getenv(endpointEnvVar)
		if url == "" {
			return OmniContext{}, fmt.Errorf("set %s when using %s", endpointEnvVar, envKey)
		}

		return OmniContext{URL: url, ServiceAcct: true}, nil
	}

	contextName := conf.Context
	if opts.Context != "" {
		contextName = opts.Context
	}

	configCtx, err := conf.GetContext(opts.Context)
	if err != nil {
		return OmniContext{}, fmt.Errorf("get context %q: %w", contextName, err)
	}

	if configCtx.URL == config.PlaceholderURL {
		return OmniContext{}, fmt.Errorf("context %q has not been configured, set the Omni URL in omniconfig", contextName)
	}

	url := configCtx.URL
	if endpoint := os.Getenv(endpointEnvVar); endpoint != "" {
		url = endpoint
	}

	identity := configCtx.Auth.SideroV1.Identity
	if identity == "" {
		return OmniContext{}, fmt.Errorf("context %q has no siderov1 identity; set with: omnictl config identity <email>", contextName)
	}

	return OmniContext{
		URL:         url,
		ContextName: contextName,
		Identity:    identity,
	}, nil
}
