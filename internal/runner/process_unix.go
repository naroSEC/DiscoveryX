//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

func prepare(cmd *exec.Cmd) (func() error, func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.Cancel = kill
	return func() error { return nil }, func() { _ = kill() }, nil
}
