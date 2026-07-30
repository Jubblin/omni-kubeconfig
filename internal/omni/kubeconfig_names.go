package omni

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type serviceAccountNames struct {
	Cluster string
	Context string
	Auth    string
}

func ensureServiceAccountUser(opts KubeconfigOptions) (KubeconfigOptions, error) {
	if !opts.ServiceAccount || opts.User != "" {
		return opts, nil
	}

	id, err := newUUIDV8String()
	if err != nil {
		return opts, fmt.Errorf("generate service-account user id: %w", err)
	}
	opts.User = id
	fmt.Fprintf(os.Stderr, "generated service-account user: %q\n", id)
	return opts, nil
}

func newUUIDV8String() (string, error) {
	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	// RFC 9562 UUIDv8: version 8 with implementation-defined random bits.
	u[6] = (u[6] & 0x0f) | 0x80
	u[8] = (u[8] & 0x3f) | 0x80
	return u.String(), nil
}

func normalizeServiceAccountKubeconfig(data []byte, opts KubeconfigOptions) ([]byte, error) {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return nil, fmt.Errorf("load service-account kubeconfig: %w", err)
	}

	oldName := primaryContextName(cfg)
	if oldName == "" {
		return data, nil
	}

	names, ok := resolveServiceAccountNames(oldName, opts)
	if !ok {
		return data, nil
	}

	applyServiceAccountNames(cfg, oldName, names)

	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return nil, fmt.Errorf("write normalized service-account kubeconfig: %w", err)
	}
	return out, nil
}

func resolveServiceAccountNames(omniName string, opts KubeconfigOptions) (serviceAccountNames, bool) {
	clusterPrefix, ok := omniClusterPrefix(omniName, opts.Cluster, opts.User)
	if !ok {
		return serviceAccountNames{}, false
	}

	authName := clusterPrefix + "-sa-" + opts.User
	contextName := authName
	if opts.ForceContextName != "" {
		contextName = opts.ForceContextName
	}

	return serviceAccountNames{
		Cluster: clusterPrefix,
		Context: contextName,
		Auth:    authName,
	}, true
}

func omniClusterPrefix(omniName, cluster, user string) (string, bool) {
	if user == "" {
		return "", false
	}

	suffix := "-" + user
	if !strings.HasSuffix(omniName, suffix) {
		return "", false
	}

	prefix := strings.TrimSuffix(omniName, suffix)
	clusterSuffix := "-" + cluster
	if strings.HasSuffix(prefix, clusterSuffix) || prefix == cluster {
		return prefix, true
	}

	return "", false
}

func primaryContextName(cfg *clientcmdapi.Config) string {
	if cfg.CurrentContext != "" {
		return cfg.CurrentContext
	}
	for name := range cfg.Contexts {
		return name
	}
	return ""
}

func applyServiceAccountNames(cfg *clientcmdapi.Config, oldName string, names serviceAccountNames) {
	if cluster, ok := cfg.Clusters[oldName]; ok {
		delete(cfg.Clusters, oldName)
		cfg.Clusters[names.Cluster] = cluster
	}

	if auth, ok := cfg.AuthInfos[oldName]; ok {
		delete(cfg.AuthInfos, oldName)
		cfg.AuthInfos[names.Auth] = auth
	}

	if ctx, ok := cfg.Contexts[oldName]; ok {
		delete(cfg.Contexts, oldName)
		ctx.Cluster = names.Cluster
		ctx.AuthInfo = names.Auth
		cfg.Contexts[names.Context] = ctx
	}

	if cfg.CurrentContext == oldName {
		cfg.CurrentContext = names.Context
	}
}
