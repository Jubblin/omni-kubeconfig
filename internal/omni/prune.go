package omni

import (
	"fmt"
	"os"
	"slices"
	"strings"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const omniNamePrefix = "omni-"

// pruneDestroyedOmniEntries removes Omni-sourced kubeconfig entries whose cluster
// no longer exists in live. Non-Omni contexts (e.g. minikube) are left intact.
// Returns sorted context names that were removed.
func pruneDestroyedOmniEntries(cfg *clientcmdapi.Config, live map[string]struct{}) []string {
	if cfg == nil {
		return nil
	}

	type drop struct {
		context string
		cluster string
		user    string
	}

	var drops []drop
	for name, ctx := range cfg.Contexts {
		if !shouldPruneOmniContext(cfg, name, ctx, live) {
			continue
		}
		item := drop{context: name}
		if ctx != nil {
			item.cluster = ctx.Cluster
			item.user = ctx.AuthInfo
		}
		drops = append(drops, item)
	}

	removed := make([]string, 0, len(drops))
	for _, item := range drops {
		delete(cfg.Contexts, item.context)
		removed = append(removed, item.context)
	}
	slices.Sort(removed)

	for _, item := range drops {
		if item.cluster != "" && !kubeconfigNameReferenced(cfg, item.cluster, kubeconfigClusterRef) {
			delete(cfg.Clusters, item.cluster)
		}
		if item.user != "" && !kubeconfigNameReferenced(cfg, item.user, kubeconfigUserRef) {
			delete(cfg.AuthInfos, item.user)
		}
	}

	if _, ok := cfg.Contexts[cfg.CurrentContext]; cfg.CurrentContext != "" && !ok {
		cfg.CurrentContext = ""
	}

	return removed
}

func shouldPruneOmniContext(cfg *clientcmdapi.Config, name string, ctx *clientcmdapi.Context, live map[string]struct{}) bool {
	if _, ok := live[name]; ok {
		return false
	}
	if ctx != nil {
		if _, ok := live[ctx.Cluster]; ok {
			return false
		}
		if _, ok := live[ctx.AuthInfo]; ok {
			return false
		}
	}

	id, ok := omniClusterIDForContext(cfg, name, ctx)
	if !ok {
		return false
	}
	_, stillLive := live[id]
	return !stillLive
}

func omniClusterIDForContext(cfg *clientcmdapi.Config, name string, ctx *clientcmdapi.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	if id, ok := omniIDFromSAName(ctx.Cluster); ok {
		return id, true
	}
	if id, ok := omniIDFromSAName(ctx.AuthInfo); ok {
		return id, true
	}
	if id, ok := omniIDFromSAName(name); ok {
		return id, true
	}
	if !isOmniOIDCAuth(cfg.AuthInfos[ctx.AuthInfo]) {
		return "", false
	}
	if ctx.Cluster != "" {
		return ctx.Cluster, true
	}
	return name, true
}

func omniIDFromSAName(name string) (string, bool) {
	if !strings.HasPrefix(name, omniNamePrefix) {
		return "", false
	}
	rest := name[len(omniNamePrefix):]
	if rest == "" {
		return "", false
	}
	if i := strings.Index(rest, "-sa-"); i > 0 {
		return rest[:i], true
	}
	return rest, true
}

func isOmniOIDCAuth(auth *clientcmdapi.AuthInfo) bool {
	if auth == nil || auth.Exec == nil {
		return false
	}
	if strings.Contains(auth.Exec.Command, "oidc-login") {
		return true
	}
	for _, arg := range auth.Exec.Args {
		if arg == "oidc-login" || strings.Contains(arg, "oidc-login") {
			return true
		}
	}
	return false
}

type kubeconfigRefKind int

const (
	kubeconfigClusterRef kubeconfigRefKind = iota
	kubeconfigUserRef
)

func kubeconfigNameReferenced(cfg *clientcmdapi.Config, name string, kind kubeconfigRefKind) bool {
	for _, ctx := range cfg.Contexts {
		if ctx == nil {
			continue
		}
		switch kind {
		case kubeconfigClusterRef:
			if ctx.Cluster == name {
				return true
			}
		case kubeconfigUserRef:
			if ctx.AuthInfo == name {
				return true
			}
		default:
			return false
		}
	}
	return false
}

func logPrunedClusters(names []string) {
	if len(names) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "pruned %d destroyed Omni cluster(s):\n", len(names))
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "  %s\n", name)
	}
}
