package tools

import (
	"context"
	"discoveryx/internal/config"
	"discoveryx/internal/model"
	"discoveryx/internal/runner"
	"discoveryx/internal/scope"
	"discoveryx/internal/storage"
	"discoveryx/internal/updater"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Opt-in: official binaries, loopback targets only. The default suite is offline.
func TestInstalledScannersOnLoopback(t *testing.T) {
	toolsDir := os.Getenv("DISCOVERYX_TEST_TOOLS")
	if toolsDir == "" {
		t.Skip("set DISCOVERYX_TEST_TOOLS to a verified tool manifest directory")
	}
	manifest, e := updater.ReadManifest(toolsDir)
	if e != nil {
		t.Fatal(e)
	}
	var externalHits atomic.Int32
	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { externalHits.Add(1) }))
	defer outside.Close()
	outsideURL := strings.Replace(outside.URL, "127.0.0.1", "localhost", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<html><title>Local fixture</title><a href="/inside">inside</a><a href="`+outsideURL+`/external">outside</a></html>`)
		case "/inside", "/admin":
			io.WriteString(w, "local fixture")
		case "/robots.txt":
			io.WriteString(w, "User-agent: *\nDisallow:\n")
		case "/sitemap.xml":
			io.WriteString(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	for _, d := range []string{"inputs", "raw"} {
		os.MkdirAll(filepath.Join(dir, d), 0700)
	}
	c := config.Default()
	c.CrawlDepth = 1
	c.DirectoryDepth = 1
	c.HTTPTimeout = 2 * time.Second
	c.TechRules = filepath.Join("..", "..", "configs", "tech-extensions.yaml")
	c.Wordlist = filepath.Join(dir, "fixture-wordlist.txt")
	os.WriteFile(c.Wordlist, []byte("admin\n"), 0600)
	c.TechRules, _ = filepath.Abs(c.TechRules)
	v, _ := scope.New("127.0.0.1", nil)
	hostfile := filepath.Join(dir, "inputs", "hosts.txt")
	urlfile := filepath.Join(dir, "inputs", "urls.txt")
	storage.Lines(hostfile, []string{server.URL})
	storage.Lines(urlfile, []string{server.URL})
	in := Input{Config: c, RunDir: dir, Scope: v, HostsFile: hostfile, URLsFile: urlfile, Services: []model.WebService{{URL: server.URL, Hostname: "127.0.0.1"}}, Warn: io.Discard}
	data := model.Data{}
	for _, name := range []string{"httpx", "katana", "feroxbuster"} {
		t.Run(name, func(t *testing.T) {
			installation, ok := manifest[name]
			if !ok {
				t.Fatalf("%s missing", name)
			}
			h, e := storage.Hash(installation.Path)
			if e != nil || h != installation.SHA256 {
				t.Fatal("binary integrity error")
			}
			adapter := Registry()[name]
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			flag := "-h"
			if name == "feroxbuster" {
				flag = "--help"
			}
			in.Help, e = runner.Probe(ctx, runner.OSExecutor{}, installation.Path, flag)
			if e != nil {
				t.Fatal(e)
			}
			calls, e := adapter.BuildArgs(in)
			if e != nil {
				t.Fatal(e)
			}
			for _, call := range calls {
				if e = adapter.Run(ctx, runner.OSExecutor{}, installation.Path, call, in); e != nil {
					b, _ := os.ReadFile(call.Output + ".stderr.log")
					t.Fatalf("%s: %v\n%s", name, e, b)
				}
				f, e := os.Open(call.Output)
				if e != nil {
					t.Fatal(e)
				}
				warnings := adapter.Parse(f, &data, in)
				f.Close()
				if len(warnings) > 0 {
					t.Fatal(warnings)
				}
			}
		})
	}
	if externalHits.Load() != 0 {
		t.Fatalf("out of scope loopback endpoint was contacted %d times", externalHits.Load())
	}
	if len(data.WebServices) == 0 {
		t.Fatal("httpx produced no normalized loopback web service")
	}
	t.Logf("loopback results: webservices=%d endpoints=%d external_requests=%d", len(data.WebServices), len(data.Endpoints), externalHits.Load())
}
