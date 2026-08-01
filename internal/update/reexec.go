package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// EnvJustUpdated is set on the restarted process so it skips another update prompt.
const EnvJustUpdated = "OMNI_KUBECONFIG_JUST_UPDATED"

// reexecFn replaces or spawns this process (overridable in tests).
var reexecFn = reexecSelf

// finishSelfUpdate restarts into the new binary with the same argv.
// On success the Unix path never returns (process replaced). On Windows the
// child is started and ErrRestartRequired is returned so the parent exits 0.
// On failure it prints a restart hint and returns ErrRestartRequired.
func finishSelfUpdate(stderr io.Writer, version string, args []string) error {
	if stderr == nil {
		stderr = os.Stderr
	}
	if len(args) == 0 {
		args = os.Args
	}

	exe, err := os.Executable()
	if err != nil {
		printRestartHint(stderr, version)
		return ErrRestartRequired
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fmt.Fprintf(stderr, "Restarting omni-kubeconfig %s...\n", normalizeTag(version)) //nolint:errcheck

	if err := reexecFn(exe, args, withJustUpdated(os.Environ())); err != nil {
		fmt.Fprintf(stderr, "[WARN] auto-restart failed: %v\n", err) //nolint:errcheck
		printRestartHint(stderr, version)
		return ErrRestartRequired
	}

	// Windows spawn succeeds with nil; parent must still exit.
	return ErrRestartRequired
}

func withJustUpdated(env []string) []string {
	out := make([]string, 0, len(env)+1)
	prefix := EnvJustUpdated + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, EnvJustUpdated+"=1")
}

func justUpdated() bool {
	return os.Getenv(EnvJustUpdated) == "1"
}
