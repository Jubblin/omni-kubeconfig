//go:build windows

package update

import "fmt"

// Windows auto-restart is implemented in a follow-up (spawn + exit).
func reexecSelf(exe string, args, env []string) error {
	return fmt.Errorf("auto-restart is not supported on windows yet")
}
