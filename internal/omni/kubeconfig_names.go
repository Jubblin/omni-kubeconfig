package omni

import (
	"fmt"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func normalizeServiceAccountKubeconfig(data []byte, opts KubeconfigOptions) ([]byte, error) {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return nil, fmt.Errorf("load service-account kubeconfig: %w", err)
	}

	oldName := primaryContextName(cfg)
	if oldName == "" {
		return data, nil
	}

	newName, ok := resolveServiceAccountContextName(oldName, opts)
	if !ok {
		return data, nil
	}

	renameKubeconfigEntry(cfg, oldName, newName)

	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return nil, fmt.Errorf("write normalized service-account kubeconfig: %w", err)
	}
	return out, nil
}

func resolveServiceAccountContextName(currentName string, opts KubeconfigOptions) (string, bool) {
	if opts.ForceContextName != "" {
		return opts.ForceContextName, true
	}
	return defaultServiceAccountContextName(currentName, opts.Cluster, opts.User)
}

func defaultServiceAccountContextName(contextName, cluster, user string) (string, bool) {
	suffix := "-" + user
	if !strings.HasSuffix(contextName, suffix) {
		return "", false
	}

	newName := strings.TrimSuffix(contextName, suffix)
	clusterSuffix := "-" + cluster
	if !strings.HasSuffix(newName, clusterSuffix) && newName != cluster {
		return "", false
	}

	return newName, true
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

func renameKubeconfigEntry(cfg *clientcmdapi.Config, oldName, newName string) {
	if oldName == "" || newName == "" || oldName == newName {
		return
	}

	if cluster, ok := cfg.Clusters[oldName]; ok {
		delete(cfg.Clusters, oldName)
		cfg.Clusters[newName] = cluster
	}

	if auth, ok := cfg.AuthInfos[oldName]; ok {
		delete(cfg.AuthInfos, oldName)
		cfg.AuthInfos[newName] = auth
	}

	if ctx, ok := cfg.Contexts[oldName]; ok {
		delete(cfg.Contexts, oldName)
		if ctx.Cluster == oldName {
			ctx.Cluster = newName
		}
		if ctx.AuthInfo == oldName {
			ctx.AuthInfo = newName
		}
		cfg.Contexts[newName] = ctx
	}

	if cfg.CurrentContext == oldName {
		cfg.CurrentContext = newName
	}
}
