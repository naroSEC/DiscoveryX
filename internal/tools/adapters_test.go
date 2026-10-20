package tools

import (
	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/scope"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensions(t *testing.T) {
	r := TechRules{Base: []string{"xml", "txt", "html"}, Rules: []TechRule{{Match: []string{`\bphp\b`}, Extensions: []string{"php"}}, {Match: []string{`\bjava\b`}, Extensions: []string{"jsp"}}, {Match: []string{`asp\.net`}, Extensions: []string{"asp", "aspx"}}}}
	for _, tc := range []struct {
		tech []string
		want string
	}{{nil, "html,txt,xml"}, {[]string{"PHP", "Java"}, "html,jsp,php,txt,xml"}, {[]string{"Microsoft IIS"}, "html,txt,xml"}, {[]string{"ASP.NET"}, "asp,aspx,html,txt,xml"}} {
		ext, e := Extensions(tc.tech, nil, r)
		if e != nil || strings.Join(ext, ",") != tc.want {
			t.Fatalf("%v => %v %v", tc.tech, ext, e)
		}
	}
	ext, _ := Extensions([]string{"PHP"}, []string{"PHP", ".php"}, r)
	if len(ext) != 4 {
		t.Fatal(ext)
	}
}
func TestNucleiNoAutomaticUpdate(t *testing.T) {
	c := config.Default()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inputs"), 0700)
	calls, e := Nuclei().BuildArgs(Input{Config: c, RunDir: dir, TemplatePaths: []string{"a.yaml"}})
	if e != nil {
		t.Fatal(e)
	}
	args := " " + strings.Join(calls[0].Args, " ") + " "
	for _, s := range []string{" -duc ", " -ni ", " -dr ", " -dut ", " -pt http "} {
		if !strings.Contains(args, s) {
			t.Fatal(args)
		}
	}
	for _, s := range []string{" -ut ", " -up ", " -code ", " -headless "} {
		if strings.Contains(args, s) {
			t.Fatal(args)
		}
	}
}
func TestScopeAndDuplicateParsers(t *testing.T) {
	v, _ := scope.New("fixture.test", nil)
	in := Input{Scope: v}
	d := model.Data{}
	Subfinder().Parse(strings.NewReader("api.fixture.test\napi.fixture.test\nfixture.test.attacker.test\n"), &d, in)
	if len(d.Assets) != 1 || len(d.External) != 1 {
		t.Fatal(d)
	}
	for i := 0; i < 2; i++ {
		Katana().Parse(strings.NewReader(`{"request":{"endpoint":"https://api.fixture.test/path","source":"https://fixture.test"}}`), &d, in)
	}
	if len(d.Endpoints) != 1 {
		t.Fatal(d)
	}
}
func TestHelpExactFlag(t *testing.T) {
	if Supports(" -ss smart-scan ", "-s") {
		t.Fatal("substring flag accepted")
	}
	if !Supports(" -s, -scan-type string", "-s") {
		t.Fatal("alias rejected")
	}
}

func TestScannerPerformanceDefaultsAndOverrides(t *testing.T) {
	for _, name := range []string{"feroxbuster", "nuclei"} {
		t.Run(name, func(t *testing.T) {
			c := config.Default()
			c.Wordlist = filepath.Join(t.TempDir(), "wordlist.txt")
			if err := os.WriteFile(c.Wordlist, []byte("admin\n"), 0600); err != nil {
				t.Fatal(err)
			}
			in := Input{Config: c, RunDir: t.TempDir(), TemplatePaths: []string{"fixture.yaml"}, Services: []model.WebService{{URL: "https://fixture.test"}}}
			scanner := Registry()[name]
			calls, err := scanner.BuildArgs(in)
			if err != nil {
				t.Fatal(err)
			}
			flags := []string{"--rate-limit", "-t", "--scan-limit"}
			if name == "nuclei" {
				flags = []string{"-rl", "-c", "-bs"}
			}
			for _, flag := range flags {
				for _, arg := range calls[0].Args {
					if arg == flag {
						t.Fatalf("default forced %s: %v", flag, calls[0].Args)
					}
				}
			}
			in.Config.Limits[name] = config.Limit{Rate: 17, Concurrency: 125}
			in.Config.FeroxScanLimit = 3
			calls, err = scanner.BuildArgs(in)
			if err != nil {
				t.Fatal(err)
			}
			joined := " " + strings.Join(calls[0].Args, " ") + " "
			for _, want := range []string{flags[0] + " 17 ", flags[1] + " 125 "} {
				if !strings.Contains(joined, " "+want) {
					t.Fatalf("missing override %q: %s", want, joined)
				}
			}
			if name == "feroxbuster" && !strings.Contains(joined, " --scan-limit 3 ") {
				t.Fatal(joined)
			}
		})
	}
}
