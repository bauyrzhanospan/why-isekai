//go:build windows

package secrets

import "os/exec"

func exitCode(ee *exec.ExitError) int {
	if c := ee.ExitCode(); c >= 0 {
		return c
	}
	return 1
}
