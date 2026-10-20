package parser

import (
	"discoveryx/internal/model"
	"strings"
	"testing"
)

func TestMalformedJSONL(t *testing.T) {
	n := 0
	w := JSONL(strings.NewReader("{\"host\":\"a\"}\n{bad\nnull\n{\"host\":\"b\"}\n"), func(m map[string]any, line int) { n++ })
	if n != 2 || len(w) != 2 {
		t.Fatalf("records %d warnings %v", n, w)
	}
}
func TestUnique(t *testing.T) {
	if len(Unique([]string{"a", "a", "", "b"})) != 2 {
		t.Fatal("dedup")
	}
	a, _ := Endpoint("https://EXAMPLE.com/p?a=1#x", "source", "GET", "katana", false, false)
	if a.URL != "https://example.com/p?a=1" || a.Query["a"][0] != "1" {
		t.Fatal(a)
	}
}

func TestEndpointProvenanceMerge(t *testing.T) {
	d := model.Data{}
	a, _ := Endpoint("https://example.com", "", "", "httpx", true, false)
	b, _ := Endpoint("https://EXAMPLE.com/#fragment", "https://example.com/", "GET", "katana", false, false)
	AddEndpoint(&d, a)
	AddEndpoint(&d, b)
	if len(d.Endpoints) != 1 || !d.Endpoints[0].Initial || !d.Endpoints[0].Crawled || len(d.Endpoints[0].SourceTools) != 2 {
		t.Fatal(d.Endpoints)
	}
}
