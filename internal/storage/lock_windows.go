//go:build windows

package storage

import (
	"golang.org/x/sys/windows"
	"os"
)

func fileLock(f *os.File) (func(), error) {
	ov := &windows.Overlapped{}
	h := windows.Handle(f.Fd())
	if e := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ov); e != nil {
		return nil, e
	}
	return func() { _ = windows.UnlockFileEx(h, 0, 1, 0, ov) }, nil
}
