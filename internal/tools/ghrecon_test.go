package tools

import (
	"strings"
	"testing"

	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
)

func TestGitHubRootDomain(t *testing.T) {
	for target, want := range map[string]string{"https://www.example.com/path?q=1": "example.com", "https://api.example.co.kr:8443": "example.co.kr", "www.example.com.": "example.com", "api.team.github.io": "team.github.io"} {
		got, err := RootDomain(target)
		if err != nil || got != want {
			t.Fatalf("%s: %s %v", target, got, err)
		}
	}
	for _, target := range []string{"https://127.0.0.1", "http://[::1]", "localhost", "https://co.uk"} {
		if _, err := RootDomain(target); err == nil {
			t.Fatal("unsupported root accepted", target)
		}
	}
}

func TestGHReconJSONLProvenanceAndDedup(t *testing.T) {
	c := config.Default()
	c.Target = "https://www.example.com"
	input := `{"schema_version":"1","type":"repository","value":"example/backend","confidence":0.9,"sources":[{"repository":"example/backend","path":"a.txt"}],"repository_details":{"clone_url":"https://github.com/example/backend.git"},"new_field":"ignored"}
{"schema_version":"1","type":"repository","value":"example/backend","confidence":0.9,"sources":[{"repository":"example/backend","path":"b.txt"}]}
{"schema_version":"2","type":"domain","value":"example.com"}
{broken
{"schema_version":"1","type":"domain","value":"outside.test","confidence":0.5,"sources":[]}`
	data := model.Data{}
	warnings := GHRecon().Parse(strings.NewReader(input), &data, Input{Config: c})
	if len(warnings) != 2 || len(data.GitHub) != 2 || len(data.GitHub[0].Sources) != 2 {
		t.Fatal(warnings, data.GitHub)
	}
	if data.GitHub[0].Domain != "example.com" || data.GitHub[0].Evidence != "raw/ghrecon.jsonl:1" || len(data.Assets) != 0 || len(data.Endpoints) != 0 {
		t.Fatal("GitHub data lost provenance or entered scanner inputs")
	}
	calls, err := GHRecon().BuildArgs(Input{Config: c, RunDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(calls[0].Args, " ")
	if !strings.Contains(args, "--domain example.com") || strings.Contains(args, "--token") {
		t.Fatal(args)
	}
}
