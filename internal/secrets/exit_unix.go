//go:build !windows

package secrets

import (
	"os/exec"
	"syscall"
)

// exitCode follows the shell convention: 128+signal for a killed child.
func exitCode(ee *exec.ExitError) int {
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	if c := ee.ExitCode(); c >= 0 {
		return c
	}
	return 1
}
