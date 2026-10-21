package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/updater"
)

func TestInstalledGHReconOnLoopback(t *testing.T) {
	dir := os.Getenv("DISCOVERYX_TEST_TOOLS")
	if dir == "" {
		t.Skip("set DISCOVERYX_TEST_TOOLS to verified binaries")
	}
	manifest, err := updater.ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	installed, ok := manifest["ghrecon"]
	if !ok {
		t.Fatal("GHRecon not installed")
	}
	if hash, err := storage.Hash(installed.Path); err != nil || hash != installed.SHA256 {
		t.Fatal("GHRecon binary integrity mismatch", err)
	}
	var calls atomic.Int32
	const token = "github_pat_LOOPBACK_TEST_ONLY"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !strings.Contains(r.Header.Get("Authorization"), token) {
			t.Error("GitHub credential not delivered to mock API")
		}
		w.Header().Set("Content-Type", "application/json")
		items := []any{}
		if r.URL.Path == "/search/repositories" {
			items = append(items, map[string]any{"full_name": "example/backend", "name": "backend", "html_url": "https://github.com/example/backend", "clone_url": "https://github.com/example/backend.git", "description": "example.com backend", "owner": map[string]any{"login": "example"}, "visibility": "public"})
		}
		if !strings.HasPrefix(r.URL.Path, "/search/") {
			t.Errorf("unexpected API route: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": len(items), "incomplete_results": false, "items": items})
	}))
	defer server.Close()
	c := config.Default()
	c.Target = "https://www.example.com"
	c.GHReconProfile = "quick"
	c.GHReconMaxQueries = 10
	c.HTTPTimeout = 2 * time.Second
	input := Input{Config: c, GitHubToken: token, RunDir: t.TempDir()}
	configPath := filepath.Join(input.RunDir, "github-loopback.yaml")
	if err = os.WriteFile(configPath, []byte("github:\n  api_url: "+server.URL+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	scanner := GHRecon()
	invocations, err := scanner.BuildArgs(input)
	if err != nil {
		t.Fatal(err)
	}
	call := invocations[0]
	call.Args = append(call.Args, "--config", configPath)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = scanner.Run(ctx, runner.OSExecutor{}, installed.Path, call, input); err != nil {
		b, _ := os.ReadFile(call.Output + ".stderr.log")
		t.Fatalf("native GHRecon: %v %s", err, b)
	}
	f, err := os.Open(call.Output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data := model.Data{}
	warnings := scanner.Parse(f, &data, input)
	if len(warnings) > 0 || len(data.GitHub) == 0 || calls.Load() == 0 {
		t.Fatal("no native normalized findings", warnings, calls.Load())
	}
	found := false
	for _, finding := range data.GitHub {
		if finding.Type == "repository" && finding.Value == "example/backend" && len(finding.Sources) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("repository provenance missing", data.GitHub)
	}
	t.Logf("native GHRecon: findings=%d local_api_requests=%d", len(data.GitHub), calls.Load())
}
