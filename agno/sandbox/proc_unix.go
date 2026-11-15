//go:build unix

package sandbox

import (
	"os/exec"
	"syscall"
)

// setProcessGroup runs the command in its own process group so a timeout
// kills every process it spawned, not just the direct child.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
