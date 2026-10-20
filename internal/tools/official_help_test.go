package tools

import (
	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/scope"
	"os"
	"path/filepath"
	"testing"
)

func TestArgumentsAgainstVerifiedWindowsHelp(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "inputs"), 0700)
	c := config.Default()
	c.Wordlist = filepath.Join(dir, "fixture-wordlist")
	os.WriteFile(c.Wordlist, []byte("fixture\n"), 0600)
	c.TechRules = filepath.Join("..", "..", "configs", "tech-extensions.yaml")
	v, _ := scope.New("fixture.test", nil)
	in := Input{Config: c, RunDir: dir, Scope: v, Services: []model.WebService{{URL: "https://fixture.test/", Hostname: "fixture.test"}}, Phase: "initial", TemplatePaths: []string{"fixture.yaml"}}
	for name, adapter := range Registry() {
		t.Run(name, func(t *testing.T) {
			b, e := os.ReadFile(filepath.Join("..", "..", "docs", name+"-help.txt"))
			if e != nil {
				t.Fatal(e)
			}
			in.Help = string(b)
			calls, e := adapter.BuildArgs(in)
			if e != nil {
				t.Fatal(e)
			}
			for _, call := range calls {
				for _, arg := range call.Args {
					if len(arg) > 0 && arg[0] == '-' && !Supports(in.Help, arg) {
						t.Errorf("%s unsupported by verified release: %s", name, arg)
					}
				}
			}
		})
	}
	c.WebOnly = true
	in.Config = c
	b, e := os.ReadFile(filepath.Join("..", "..", "docs", "httpx-help.txt"))
	if e != nil {
		t.Fatal(e)
	}
	calls, e := HTTPX().BuildArgs(in)
	if e != nil {
		t.Fatal(e)
	}
	for _, arg := range calls[0].Args {
		if len(arg) > 0 && arg[0] == '-' && !Supports(string(b), arg) {
			t.Errorf("httpx web-only unsupported flag: %s", arg)
		}
	}
}
