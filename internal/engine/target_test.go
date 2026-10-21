package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/testutil"
	"github.com/naroSEC/DiscoveryX/internal/tools"
)

func TestInitializeRootDomainTarget(t *testing.T) {
	for _, tc := range []struct {
		target, want string
		webOnly      bool
	}{
		{"https://www.example.com/app?q=1", "example.com", false},
		{"api.example.co.kr", "example.co.kr", false},
		{"https://api.team.github.io", "team.github.io", false},
		{"www.example.com.", "example.com", false},
		{"https://127.0.0.1:8443/app", "127.0.0.1", false},
		{"http://[::1]:8080/app", "::1", false},
		{"localhost", "localhost", false},
		{"https://www.example.com:8443/app?q=1", "https://www.example.com:8443/app?q=1", true},
	} {
		t.Run(tc.target, func(t *testing.T) {
			c := config.Default()
			c.Target, c.WebOnly, c.GHRecon = tc.target, tc.webOnly, "off"
			c.Output = t.TempDir()
			e := New(c, io.Discard)
			if err := e.Initialize(); err != nil {
				t.Fatal(err)
			}
			if e.Config.Target != tc.want || e.Meta.Config.Target != tc.want || e.Meta.OriginalTarget != tc.target {
				t.Fatalf("target=%q saved=%q original=%q", e.Config.Target, e.Meta.Config.Target, e.Meta.OriginalTarget)
			}
			if !tc.webOnly && tc.want == "example.com" {
				if !e.Meta.Scope.Contains("api.example.com") || e.Meta.Scope.Contains("example.com.attacker.test") {
					t.Fatal("root-domain boundaries incorrect")
				}
				args, err := tools.Subfinder().BuildArgs(tools.Input{Config: e.Config, RunDir: e.Dir})
				if err != nil || !strings.Contains(strings.Join(args[0].Args, " "), "-d example.com ") {
					t.Fatalf("subfinder args: %+v %v", args, err)
				}
			}
			if tc.webOnly && e.Meta.Scope.Contains("api.example.com") {
				t.Fatal("web-only scope expanded")
			}
			resumed, err := Resume(e.Dir, io.Discard)
			if err != nil || resumed.Config.Target != tc.want {
				t.Fatalf("resume changed target: %v", err)
			}
		})
	}
}

func TestRootTargetPipelineAndInitialNaabu(t *testing.T) {
	e := fixture(t, testutil.Binary(t))
	c := e.Config
	c.Target = "https://www.fixture.test/app?q=1"
	c.PortScan, c.Crawl, c.Directory, c.Nuclei = "first", "off", "off", "off"
	e = New(c, io.Discard)
	if err := e.Initialize(); err != nil {
		t.Fatal(err)
	}
	var resolved []string
	e.ResolveHost = func(_ context.Context, host string) ([]string, error) {
		resolved = append(resolved, host)
		return []string{"127.0.0.1"}, nil
	}
	if err := e.Run(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 || resolved[0] != "fixture.test" {
		t.Fatalf("initial naabu resolved %v", resolved)
	}
	if len(e.State.Data.Ports) != 1 || e.State.Data.Ports[0].Host != "fixture.test" {
		t.Fatalf("port attribution: %+v", e.State.Data.Ports)
	}
	for _, name := range []string{"dnsx-hosts.txt", "httpx-hosts.txt"} {
		b, err := os.ReadFile(filepath.Join(e.Dir, "inputs", name))
		for _, host := range []string{"fixture.test", "www.fixture.test", "api.fixture.test"} {
			if err != nil || !strings.Contains("\n"+string(b), "\n"+host+"\n") {
				t.Fatalf("%s missing %s: %s %v", name, host, b, err)
			}
		}
	}
	for _, cmd := range e.State.Commands {
		if cmd.Tool == "subfinder" && strings.Contains(strings.Join(cmd.Args, " "), "-d www.") {
			t.Fatal("subfinder received the original hostname")
		}
	}
}

func TestRootTargetRespectsExcludedInput(t *testing.T) {
	c := config.Default()
	c.Target, c.GHRecon = "https://www.example.com", "off"
	c.Excluded = []string{"www.example.com"}
	c.Output = t.TempDir()
	if err := New(c, io.Discard).Initialize(); err == nil {
		t.Fatal("excluded original input accepted after root normalization")
	}
}
