//go:build !windows

package update

import "syscall"

func reexecSelf(exe string, args, env []string) error {
	return syscall.Exec(exe, args, env)
}
