//go:build windows

package update

import (
	"os"
	"os/exec"
)

// reexecSelf starts a new process with the updated binary and returns nil so the
// parent can exit (finishSelfUpdate then returns ErrRestartRequired). The child
// inherits JUST_UPDATED via env and continues the original command.
func reexecSelf(exe string, args, env []string) error {
	cmd := exec.Command(exe, stripArgv0(args)...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}
	return cmd.Start()
}
