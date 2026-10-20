package testutil

import (
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/updater"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func Binary(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	dest := filepath.Join(t.TempDir(), "모의 도구 경로")
	if e := os.MkdirAll(dest, 0700); e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(dest, "scanner")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, "./tests/mockscanner")
	cmd.Dir = root
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build mock: %v %s", e, b)
	}
	return exe
}
func Install(t *testing.T, binary, dir string) updater.Manifest {
	t.Helper()
	if e := os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(binary)
	if e != nil {
		t.Fatal(e)
	}
	m := updater.Manifest{}
	for _, name := range updater.Names {
		p := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			p += ".exe"
		}
		if e = os.WriteFile(p, b, 0700); e != nil {
			t.Fatal(e)
		}
		h, e := storage.Hash(p)
		if e != nil {
			t.Fatal(e)
		}
		m[name] = updater.Installation{Name: name, Repository: updater.Repositories[name], Path: p, Version: "1.2.3", Tag: "v1.2.3", SHA256: h}
	}
	if e = storage.JSON(filepath.Join(dir, "manifest.json"), m); e != nil {
		t.Fatal(e)
	}
	return m
}
