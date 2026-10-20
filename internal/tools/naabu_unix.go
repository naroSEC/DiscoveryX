//go:build !windows

package tools

import "os"

func SYNAvailable() error {
	if os.Geteuid() != 0 {
		return platformError()
	}
	return nil
}
