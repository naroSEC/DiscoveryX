//go:build windows

package runner

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
	"unsafe"
)

func prepare(cmd *exec.Cmd) (func() error, func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	job, e := windows.CreateJobObject(nil, nil)
	if e != nil {
		return nil, nil, e
	}
	cleanup := func() { windows.CloseHandle(job) }
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, e = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); e != nil {
		cleanup()
		return nil, nil, e
	}
	cmd.Cancel = func() error {
		_ = windows.TerminateJobObject(job, 1)
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	attach := func() error {
		proc, e := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(cmd.Process.Pid))
		if e != nil {
			return e
		}
		defer windows.CloseHandle(proc)
		if e = windows.AssignProcessToJobObject(job, proc); e != nil {
			var code uint32
			if qe := windows.GetExitCodeProcess(proc, &code); qe == nil && code != 259 {
				return nil
			}
			return fmt.Errorf("cannot isolate child process in Windows job: %w", e)
		}
		return nil
	}
	return attach, cleanup, nil
}
