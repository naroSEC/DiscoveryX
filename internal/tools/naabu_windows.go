//go:build windows

package tools

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func SYNAvailable() error {
	var sid *windows.SID
	var e error
	sid, e = windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if e != nil {
		return e
	}
	member, e := windows.Token(0).IsMember(sid)
	if e != nil || !member {
		return fmt.Errorf("SYN requires an elevated administrator token")
	}
	for _, p := range []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "Npcap", "wpcap.dll"), filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "npcap.sys")} {
		if _, e = os.Stat(p); e != nil {
			return fmt.Errorf("Npcap unavailable: %s", p)
		}
	}
	return nil
}
