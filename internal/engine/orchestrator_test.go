package engine

import (
	"context"
	"discoveryx/internal/config"
	"discoveryx/internal/scope"
	"discoveryx/internal/testutil"
	"discoveryx/internal/tools"
	"discoveryx/internal/updater"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, exe string) *Engine {
	t.Helper()
	dir := t.TempDir()
	c := config.Default()
	c.Target = "fixture.test"
	c.PortScan = "off"
	c.Output = filepath.Join(dir, "결과 공백")
	c.ToolsDir = filepath.Join(dir, "tools")
	c.TemplatesDir = filepath.Join(dir, "templates")
	c.Wordlist = filepath.Join(dir, "wordlist.txt")
	c.TechRules = filepath.Join(dir, "rules.yaml")
	c.StageTimeout = 5 * time.Second
	c.MaxDuration = time.Minute
	os.WriteFile(c.Wordlist, []byte("admin\n"), 0600)
	os.WriteFile(c.TechRules, []byte("base: [xml,txt,html]\nrules:\n  - match: ['php']\n    extensions: [php]\n"), 0600)
	os.MkdirAll(c.TemplatesDir, 0700)
	os.WriteFile(filepath.Join(c.TemplatesDir, "fixture.yaml"), []byte("id: fixture\ninfo:\n  name: Fixture\n  severity: high\n  tags: cve,kev\nhttp:\n  - method: GET\n    path: ['{{BaseURL}}/test']\n    matchers:\n      - type: status\n        status: [200]\n# digest: 00:00\n"), 0600)
	testutil.Install(t, exe, c.ToolsDir)
	e := New(c, io.Discard)
	if er := e.Initialize(); er != nil {
		t.Fatal(er)
	}
	return e
}
func TestOfflinePipelinePartialAndResume(t *testing.T) {
	exe := testutil.Binary(t)
	eng := fixture(t, exe)
	log := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("MOCK_RECORD", log)
	t.Setenv("MOCK_FAIL_TOOL", "katana")
	if er := eng.Run(context.Background(), false); er != nil {
		t.Fatal(er)
	}
	if eng.Meta.State != "Partial" {
		t.Fatal(eng.Meta.State)
	}
	d := eng.State.Data
	if len(d.Assets) != 2 || len(d.WebServices) != 2 || len(d.Vulnerabilities) != 2 || d.Vulnerabilities[1].DuplicateOf == "" {
		t.Fatalf("bad normalized results: %+v", d)
	}
	if len(d.InvalidHosts) != 0 {
		t.Fatal(d.InvalidHosts)
	}
	for _, name := range []string{"dnsx-hosts.txt", "httpx-hosts.txt"} {
		b, er := os.ReadFile(filepath.Join(eng.Dir, "inputs", name))
		if er != nil || strings.Contains(string(b), "attacker") {
			t.Fatalf("%s: %s %v", name, b, er)
		}
	}
	callsBefore, _ := os.ReadFile(log)
	t.Setenv("MOCK_FAIL_TOOL", "")
	resumed, er := Resume(eng.Dir, io.Discard)
	if er != nil {
		t.Fatal(er)
	}
	if er = resumed.Run(context.Background(), true); er != nil {
		t.Fatal(er)
	}
	if resumed.Meta.State != "Completed" {
		t.Fatalf("%s %+v", resumed.Meta.State, resumed.State.Stages)
	}
	callsAfter, _ := os.ReadFile(log)
	newCalls := strings.TrimPrefix(string(callsAfter), string(callsBefore))
	if strings.Contains(newCalls, "subfinder ") || strings.Contains(newCalls, "dnsx ") || strings.Contains(newCalls, "httpx ") {
		t.Fatalf("completed prefix rerun: %s", newCalls)
	}
	if !strings.Contains(newCalls, "katana ") {
		t.Fatal("failed stage not retried")
	}
	if strings.Contains(string(callsAfter), " -ut ") {
		t.Fatal("scan updated templates")
	}
	// Corrupt a required output; status alone must not qualify as completed.
	os.WriteFile(filepath.Join(eng.Dir, "raw", "dnsx.jsonl"), []byte("corrupt\n"), 0600)
	resumed, er = Resume(eng.Dir, io.Discard)
	if er != nil {
		t.Fatal(er)
	}
	if er = resumed.Run(context.Background(), true); er != nil {
		t.Fatal(er)
	}
	b, _ := os.ReadFile(filepath.Join(eng.Dir, "raw", "dnsx.jsonl"))
	if string(b) == "corrupt\n" {
		t.Fatal("corrupt checkpoint reused")
	}
	matches, _ := filepath.Glob(filepath.Join(eng.Dir, "raw", "dnsx.jsonl.previous-*"))
	if len(matches) == 0 {
		t.Fatal("previous raw result was not preserved")
	}
}
func TestRootDomainWithoutSubfinder(t *testing.T) {
	e := fixture(t, testutil.Binary(t))
	e.Config.Subfinder = "off"
	e.Config.Crawl = "off"
	e.Config.Directory = "off"
	e.Config.Nuclei = "off"
	e.Meta.Config = e.Config
	e.State.Stages = nil
	for _, s := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, stageState(s.Name))
	}
	if er := e.Run(context.Background(), false); er != nil {
		t.Fatal(er)
	}
	if len(e.State.Data.WebServices) != 1 || e.State.Data.WebServices[0].Hostname != "fixture.test" {
		t.Fatal(e.State.Data)
	}
	b, err := os.ReadFile(filepath.Join(e.Dir, "inputs", "web-urls.txt"))
	if err != nil || !strings.Contains(string(b), "https://fixture.test/") {
		t.Fatalf("live URL file missing after final httpx stage: %s %v", b, err)
	}
}
func TestWebPreflightLocalNoRedirect(t *testing.T) {
	localHits := 0
	externalHits := 0
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { externalHits++ }))
	defer external.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localHits++
		if r.Method != "HEAD" {
			t.Error("preflight should use HEAD")
		}
		http.Redirect(w, r, external.URL, 302)
	}))
	defer local.Close()
	v, _ := scope.New("127.0.0.1", nil)
	got, er := WebTarget(context.Background(), local.URL+"/keep?x=1", time.Second, v)
	if er != nil || got != local.URL+"/keep?x=1" || externalHits != 0 {
		t.Fatalf("%s %v hits=%d", got, er, externalHits)
	}
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	got, er = probeWeb(context.Background(), []string{local.URL}, client, v)
	if er != nil || !strings.HasPrefix(got, local.URL) || localHits != 1 || externalHits != 0 {
		t.Fatalf("preflight %s %v local=%d external=%d", got, er, localHits, externalHits)
	}
}
func TestResumeRejectsBinaryChange(t *testing.T) {
	e := fixture(t, testutil.Binary(t))
	e.Config.Crawl = "off"
	e.Config.Directory = "off"
	e.Config.Nuclei = "off"
	e.Meta.Config = e.Config
	e.State.Stages = nil
	for _, s := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, stageState(s.Name))
	}
	if er := e.Run(context.Background(), false); er != nil {
		t.Fatal(er)
	}
	manifest, er := updater.ReadManifest(e.Config.ToolsDir)
	if er != nil {
		t.Fatal(er)
	}
	os.WriteFile(manifest["dnsx"].Path, []byte("tampered"), 0600)
	r, er := Resume(e.Dir, io.Discard)
	if er != nil {
		t.Fatal(er)
	}
	if er = r.Run(context.Background(), true); er == nil {
		t.Fatal("changed executable accepted")
	}
}

func TestPortBothOnlyNewResolvedIPs(t *testing.T) {
	exe := testutil.Binary(t)
	e := fixture(t, exe)
	e.Config.PortScan = "both"
	e.Config.Crawl = "off"
	e.Config.Directory = "off"
	e.Config.Nuclei = "off"
	e.Meta.Config = e.Config
	e.Meta.Scope, _ = scope.New(e.Config.Target, nil)
	e.State.Stages = nil
	for _, s := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, stageState(s.Name))
	}
	e.ResolveHost = func(_ context.Context, h string) ([]string, error) {
		if h == "api.fixture.test" {
			return []string{"127.0.0.2"}, nil
		}
		return []string{"127.0.0.1"}, nil
	}
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(e.State.Data.Ports) != 2 {
		t.Fatalf("ports %+v stages %+v", e.State.Data.Ports, e.State.Stages)
	}
	if e.State.Data.Ports[0].Phase != "initial" || e.State.Data.Ports[1].Phase != "final" || e.State.Data.Ports[1].Host != "api.fixture.test" {
		t.Fatal(e.State.Data.Ports)
	}
	b, err := os.ReadFile(filepath.Join(e.Dir, "inputs", "naabu_final-hosts.txt"))
	if err != nil || strings.Contains(string(b), "127.0.0.1") {
		t.Fatalf("final duplicated initial: %s %v", b, err)
	}
}

func TestNucleiZeroLoadedCannotComplete(t *testing.T) {
	e := fixture(t, testutil.Binary(t))
	t.Setenv("MOCK_ZERO_TEMPLATES", "1")
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	last := e.State.Stages[len(e.State.Stages)-1]
	if last.Name != "nuclei" || last.Status != "Failed" || !strings.Contains(last.Reason, "loaded signed templates") {
		t.Fatalf("zero templates reported success: %+v", last)
	}
	if len(e.State.Data.Vulnerabilities) != 0 {
		t.Fatal("zero loaded templates produced findings")
	}
}

func webOnlyFixture(t *testing.T, exe string) *Engine {
	t.Helper()
	e := fixture(t, exe)
	e.Config.Target = "http://fixture.test:8080/app?x=1"
	e.Config.WebOnly = true
	e.Config.PortScan = "both"
	e.Config.Crawl = "off"
	e.Config.Nuclei = "off"
	e.Meta.Config = e.Config
	e.Meta.Scope, _ = scope.New(e.Config.Target, e.Config.Excluded)
	e.State.Stages = nil
	for _, s := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, stageState(s.Name))
	}
	return e
}

func TestWebOnlyFingerprintingFeedsDirectoryExtensionsAndResumes(t *testing.T) {
	e := webOnlyFixture(t, testutil.Binary(t))
	record := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("MOCK_RECORD", record)
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.Meta.State != "Completed" {
		t.Fatalf("%s %+v", e.Meta.State, e.State.Stages)
	}
	if len(e.State.Data.WebServices) != 1 {
		t.Fatal(e.State.Data.WebServices)
	}
	w := e.State.Data.WebServices[0]
	if w.URL != e.Config.Target || w.Tool != "httpx" || w.Status != 200 || len(w.Technologies) != 1 || w.Technologies[0] != "PHP" {
		t.Fatal(w)
	}
	if e.State.Stages[0].Count != 1 {
		t.Fatal("enriched input URL not counted")
	}
	hosts, err := os.ReadFile(filepath.Join(e.Dir, "inputs", "httpx-hosts.txt"))
	if err != nil || string(hosts) != e.Config.Target+"\n" {
		t.Fatalf("full URL was not preserved: %s %v", hosts, err)
	}
	calls, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	text := string(calls)
	if !strings.Contains(text, "httpx ") || !strings.Contains(text, " -nfs ") || !strings.Contains(text, "-x html,php,txt,xml") {
		t.Fatal(text)
	}
	for _, tool := range []string{"naabu ", "subfinder ", "dnsx ", "katana ", "nuclei "} {
		if strings.Contains(text, tool) {
			t.Fatalf("unexpected web-only tool: %s", text)
		}
	}
	r, err := Resume(e.Dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(record)
	if string(after) != text {
		t.Fatal("completed web-only stages rerun")
	}
	if r.State.Data.WebServices[0].Technologies[0] != "PHP" {
		t.Fatal("technology metadata lost on resume")
	}
}

func TestWebOnlyHTTPXFailureKeepsInputURL(t *testing.T) {
	e := webOnlyFixture(t, testutil.Binary(t))
	t.Setenv("MOCK_FAIL_TOOL", "httpx")
	t.Setenv("MOCK_EMPTY_TOOL", "httpx")
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.State.Stages[0].Status != "Failed" || e.State.Stages[1].Status != "Completed" {
		t.Fatal(e.State.Stages)
	}
	if len(e.State.Data.WebServices) != 1 || e.State.Data.WebServices[0].URL != e.Config.Target || len(e.State.Data.WebServices[0].Technologies) != 0 {
		t.Fatal(e.State.Data.WebServices)
	}
}

func TestInstalledWebOnlyFingerprintOnLoopback(t *testing.T) {
	toolsDir := os.Getenv("DISCOVERYX_TEST_TOOLS")
	if toolsDir == "" {
		t.Skip("set DISCOVERYX_TEST_TOOLS to a verified tool manifest directory")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("X-Powered-By", "PHP/8.3.0")
		io.WriteString(w, "<html><title>Local PHP fixture</title></html>")
	}))
	defer server.Close()
	dir := t.TempDir()
	c := config.Default()
	c.Target = server.URL + "/app?x=1"
	c.WebOnly = true
	c.Crawl = "off"
	c.Directory = "off"
	c.Nuclei = "off"
	c.ToolsDir = toolsDir
	c.Output = filepath.Join(dir, "runs")
	c.TemplatesDir = filepath.Join(dir, "templates")
	c.Wordlist = filepath.Join(dir, "fixture-wordlist.txt")
	if err := os.WriteFile(c.Wordlist, []byte("admin\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var err error
	c.TechRules, err = filepath.Abs(filepath.Join("..", "..", "configs", "tech-extensions.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	e := New(c, io.Discard)
	if err = e.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err = e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if e.Meta.State != "Completed" || len(e.State.Data.WebServices) != 1 {
		t.Fatalf("%s %+v", e.Meta.State, e.State.Stages)
	}
	w := e.State.Data.WebServices[0]
	if w.URL != c.Target || w.Title != "Local PHP fixture" || w.Status != 200 {
		t.Fatal(w)
	}
	foundPHP := false
	for _, tech := range w.Technologies {
		if strings.EqualFold(tech, "php") || strings.HasPrefix(strings.ToLower(tech), "php:") {
			foundPHP = true
		}
	}
	if !foundPHP {
		t.Fatalf("PHP fingerprint missing: %+v", w)
	}
	calls, err := tools.Feroxbuster().BuildArgs(tools.Input{Config: e.Config, RunDir: e.Dir, Scope: e.Meta.Scope, Services: e.State.Data.WebServices})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || !strings.Contains(strings.Join(calls[0].Args, " "), "html,php,txt,xml") {
		t.Fatal(calls)
	}
	t.Logf("single URL preserved: %s; technologies=%v; PHP directory extension selected", w.URL, w.Technologies)
}
