package tools

import (
	"context"
	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/testutil"
	"os"
	"path/filepath"
	"testing"
)

const goodTemplate = "id: fixture\ninfo:\n  name: fixture\n  severity: high\n  tags: cve,kev\nhttp:\n  - method: GET\n    path: ['{{BaseURL}}/test']\n    matchers:\n      - type: status\n        status: [200]\n# digest: 00:00\n"

func TestTemplatePolicy(t *testing.T) {
	c := config.Default()
	c.TemplatesDir = t.TempDir()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(c.TemplatesDir, "safe.yaml"), []byte(goodTemplate), 0600)
	for name, body := range map[string]string{"remote.yaml": "id: remote\ninfo: {tags: cve, severity: high}\nhttp:\n  - method: GET\n    path: ['https://outside.test/']\n", "post.yaml": "id: post\ninfo: {tags: cve, severity: high}\nhttp:\n  - method: POST\n    path: ['{{BaseURL}}/']\n", "oast.yaml": "id: oast\ninfo: {tags: 'cve,oast', severity: high}\nhttp: []\n"} {
		os.WriteFile(filepath.Join(c.TemplatesDir, name), []byte(body), 0600)
	}
	paths, report, e := SelectedTemplates(Input{Config: c, RunDir: dir})
	if e != nil {
		t.Fatal(e)
	}
	if len(paths) != 1 || report.Rejected != 3 || report.Tags["cve"] != 4 || report.Tags["kev"] != 1 || report.Tags["vkev"] != 0 {
		t.Fatalf("%v %+v", paths, report)
	}
	c.Tags = "vkev"
	paths, _, e = SelectedTemplates(Input{Config: c, RunDir: dir})
	if e != nil || len(paths) != 0 {
		t.Fatalf("%v %v", paths, e)
	}
}
func TestTemplateUpdateRollback(t *testing.T) {
	exe := testutil.Binary(t)
	dir := filepath.Join(t.TempDir(), "templates")
	s, e := UpdateTemplates(context.Background(), runner.OSExecutor{}, exe, dir)
	if e != nil || s.Files != 1 {
		t.Fatalf("%+v %v", s, e)
	}
	hash, _, e := Tree(dir)
	if e != nil || hash != s.TreeSHA256 {
		t.Fatalf("%s %v", hash, e)
	}
	before, e := storage.Hash(filepath.Join(dir, "fixture.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("MOCK_UPDATE_FAIL", "1")
	if _, e = UpdateTemplates(context.Background(), runner.OSExecutor{}, exe, dir); e == nil {
		t.Fatal("failed update accepted")
	}
	after, _ := storage.Hash(filepath.Join(dir, "fixture.yaml"))
	if before != after {
		t.Fatal("existing templates changed")
	}
}

func TestLoadedTemplateCount(t *testing.T) {
	for _, tc := range []struct {
		log   string
		n     int
		known bool
	}{{"[INF] Templates loaded for current scan: 12", 12, true}, {"[INF] Templates loaded for current scan: 0", 0, true}, {"Listing available templates\nfixture.yaml\n", 0, false}} {
		n, known := LoadedTemplates(tc.log)
		if n != tc.n || known != tc.known {
			t.Fatalf("%q => %d %t", tc.log, n, known)
		}
	}
}
