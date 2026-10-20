//go:build !windows

package runner_test

import "syscall"

func processAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }
