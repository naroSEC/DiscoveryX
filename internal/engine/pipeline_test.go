package engine

import (
	"discoveryx/internal/config"
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
	if len(p) != 3 || p[0].Name != "katana" || p[2].Name != "nuclei" {
		t.Fatal(p)
	}
	c.Crawl = "off"
	if len(Plan(c)) != 2 {
		t.Fatal("web-only must honor disable flags")
	}
	c = config.Default()
	c.Subfinder = "off"
	p = Plan(c)
	if p[1].Name != "dnsx" || p[2].Name != "httpx" {
		t.Fatal(p)
	}
}
