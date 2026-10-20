package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicAndOSLock(t *testing.T) {
	dir := t.TempDir()
	unlock, e := Lock(dir)
	if e != nil {
		t.Fatal(e)
	}
	if release, e := Lock(dir); e == nil {
		release()
		t.Fatal("concurrent lock accepted")
	}
	unlock()
	release, e := Lock(dir)
	if e != nil {
		t.Fatal("stale marker prevented restart", e)
	}
	release()
	p := filepath.Join(dir, "한글 공백.json")
	if e = JSON(p, map[string]int{"v": 1}); e != nil {
		t.Fatal(e)
	}
	if e = JSON(p, map[string]int{"v": 2}); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
}
