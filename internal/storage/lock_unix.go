//go:build !windows

package storage

import (
	"golang.org/x/sys/unix"
	"os"
)

func fileLock(f *os.File) (func(), error) {
	if e := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		return nil, e
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }, nil
}
