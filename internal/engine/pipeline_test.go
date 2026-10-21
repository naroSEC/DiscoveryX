package engine

import (
	"github.com/naroSEC/DiscoveryX/internal/config"
	"reflect"
	"testing"
)

func TestPlans(t *testing.T) {
	for mode, want := range map[string][]string{"first": {"naabu_initial", "subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei"}, "last": {"subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei", "naabu_final"}, "both": {"naabu_initial", "subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei", "naabu_final"}, "off": {"subfinder", "dnsx", "httpx", "katana", "feroxbuster", "nuclei"}} {
		c := config.Default()
		c.PortScan = mode
		var got []string
		for _, s := range Plan(c) {
			got = append(got, s.Name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v", mode, got)
		}
	}
	c := config.Default()
	c.WebOnly = true
	c.PortScan = "both"
	p := Plan(c)
	if len(p) != 4 || p[0].Name != "httpx" || p[1].Name != "katana" || p[3].Name != "nuclei" {
		t.Fatal(p)
	}
	c.Crawl = "off"
	if len(Plan(c)) != 3 {
		t.Fatal("web-only must honor disable flags")
	}
	c = config.Default()
	c.Subfinder = "off"
	p = Plan(c)
	if p[1].Name != "dnsx" || p[2].Name != "httpx" {
		t.Fatal(p)
	}
}

func TestGitHubPlanPosition(t *testing.T) {
	for _, webOnly := range []bool{true, false} {
		c := config.Default()
		c.GHRecon = "on"
		c.WebOnly = webOnly
		plan := Plan(c)
		found := false
		for i, stage := range plan {
			if stage.Name == "ghrecon" {
				found = true
				if i == 0 || plan[i-1].Name != "httpx" || plan[i+1].Name != "katana" {
					t.Fatal(plan)
				}
			}
		}
		if !found {
			t.Fatal("GitHub stage absent")
		}
	}
}
