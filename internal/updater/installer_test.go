package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeExecutor struct{ fail bool }

func (e *fakeExecutor) Run(ctx context.Context, r runner.Request) error {
	if e.fail {
		return fmt.Errorf("candidate invalid")
	}
	if r.Args[0] == "-h" {
		fmt.Fprint(r.Stdout, " -duc disable-update-check ")
		return nil
	}
	fmt.Fprint(r.Stdout, "naabu 1.2.3")
	return nil
}
func zipBytes(t *testing.T, name string, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, e := z.Create(name)
	if e != nil {
		t.Fatal(e)
	}
	w.Write(b)
	z.Close()
	return buf.Bytes()
}
func TestVersionCompareAndAssets(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"v2.10.0", "2.9.9", 1}, {"1.2.3", "v1.2.3", 0}, {"v1.0.0", "2.0.0", -1}} {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Fatalf("%s %s => %d", tc.a, tc.b, got)
		}
	}
	r := Release{Tag: "v1", Assets: []Asset{{Name: "x86_64-windows-debug-feroxbuster.exe.zip"}, {Name: "x86_64-windows-feroxbuster.exe.zip"}}}
	a, e := SelectAsset(r, "feroxbuster", "windows", "amd64")
	if e != nil || strings.Contains(a.Name, "debug") {
		t.Fatalf("%v %v", a, e)
	}
	if _, e = SelectAsset(r, "feroxbuster", "windows", "arm64"); e == nil {
		t.Fatal("unsupported arch accepted")
	}
}
func TestExtractTraversal(t *testing.T) {
	for _, name := range []string{"../naabu.exe", "C:/naabu.exe", `..\naabu.exe`} {
		if _, e := Extract("a.zip", "naabu.exe", zipBytes(t, name, []byte("x"))); e == nil {
			t.Fatal(name)
		}
	}
	b, e := Extract("a.zip", "naabu.exe", zipBytes(t, "bin/naabu.exe", []byte("ok")))
	if e != nil || string(b) != "ok" {
		t.Fatalf("%s %v", b, e)
	}
}
func TestInstallNoRedownloadRollbackChecksum(t *testing.T) {
	exe := "naabu"
	suffix := ".tar.gz"
	if runtime.GOOS == "windows" {
		exe += ".exe"
		suffix = ".zip"
	}
	if runtime.GOOS != "windows" {
		suffix = ".zip"
	}
	archive := zipBytes(t, exe, []byte("mock binary"))
	h := sha256.Sum256(archive)
	digest := hex.EncodeToString(h[:])
	tag := "v1.2.3"
	downloads := 0
	apiCalls := 0
	badChecksum := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases"):
			apiCalls++
			d := digest
			if badChecksum {
				d = strings.Repeat("0", 64)
			}
			json.NewEncoder(w).Encode([]Release{{Tag: "v99.0.0-beta", Prerelease: true}, {Tag: tag, Assets: []Asset{{Name: "naabu_" + strings.TrimPrefix(tag, "v") + "_" + runtime.GOOS + "_" + runtime.GOARCH + suffix, URL: server.URL + "/asset", Digest: "sha256:" + d}}}})
		case r.URL.Path == "/asset":
			downloads++
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ex := &fakeExecutor{}
	m := &Manager{Dir: t.TempDir(), GitHub: &GitHub{Base: server.URL, Client: server.Client()}, Executor: ex, Log: io.Discard}
	first, e := m.Ensure(context.Background(), "naabu", false)
	if e != nil {
		t.Fatal(e)
	}
	if downloads != 1 || first.Version != "1.2.3" {
		t.Fatal(first)
	}
	_, e = m.Ensure(context.Background(), "naabu", false)
	if e != nil || downloads != 1 || apiCalls != 1 {
		t.Fatalf("reinstalled existing binary %d %d %v", downloads, apiCalls, e)
	}
	tag = "v2.0.0"
	before, _ := os.ReadFile(filepath.Join(m.Dir, "manifest.json"))
	_, e = m.Ensure(context.Background(), "naabu", true)
	if e == nil {
		t.Fatal("wrong candidate version accepted")
	}
	after, _ := os.ReadFile(filepath.Join(m.Dir, "manifest.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("manifest changed on failure")
	}
	hash, _ := storage.Hash(first.Path)
	if hash != first.SHA256 {
		t.Fatal("old binary changed")
	}
	badChecksum = true
	if _, e = m.Ensure(context.Background(), "naabu", true); e == nil || !strings.Contains(e.Error(), "SHA256 mismatch") {
		t.Fatalf("checksum not enforced: %v", e)
	}
}
func TestHTTPRetryAndCancellation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(500)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()
	g := &GitHub{Base: server.URL, Client: server.Client()}
	b, e := g.Get(context.Background(), server.URL, 100)
	if e != nil || string(b) != "ok" || calls != 2 {
		t.Fatalf("%s %v calls=%d", b, e, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = g.Get(ctx, server.URL, 100); e == nil {
		t.Fatal("cancellation ignored")
	}
}
