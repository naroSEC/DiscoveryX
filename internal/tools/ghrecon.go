package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/parser"
	"github.com/naroSEC/DiscoveryX/internal/scope"
	"golang.org/x/net/publicsuffix"
)

func RootDomain(target string) (string, error) {
	host, err := scope.Host(target)
	if err != nil {
		return "", err
	}
	if net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return "", fmt.Errorf("GHRecon needs a domain name; IP and single-label targets are unsupported")
	}
	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", fmt.Errorf("derive GHRecon root domain: %w", err)
	}
	return domain, nil
}

func GHRecon() Tool {
	return Adapter{name: "ghrecon", inputs: []string{"root_domain", "github_token"}, build: func(in Input) ([]Invocation, error) {
		domain, err := RootDomain(in.Config.Target)
		if err != nil {
			return nil, err
		}
		args := []string{"--domain", domain, "--profile", in.Config.GHReconProfile, "--jsonl", "--verbose", "--max-queries", num(in.Config.GHReconMaxQueries), "--max-results", num(in.Config.GHReconMaxResults), "--concurrency", num(in.Config.GHReconConcurrency), "--timeout", in.Config.HTTPTimeout.String()}
		return []Invocation{{Args: args, Output: raw(in, "ghrecon")}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		domain, _ := RootDomain(in.Config.Target)
		var issues []string
		warnings := parser.JSONL(r, func(m map[string]any, line int) {
			b, _ := json.Marshal(m)
			var f model.GitHubFinding
			if err := json.Unmarshal(b, &f); err != nil || f.SchemaVersion != "1" || f.Type == "" || f.Value == "" || f.Confidence < 0 || f.Confidence > 1 {
				issues = append(issues, fmt.Sprintf("ghrecon line %d: unsupported or malformed schema-v1 finding", line))
				return
			}
			f.Domain = domain
			f.Tool = "ghrecon"
			f.Evidence = fmt.Sprintf("raw/ghrecon.jsonl:%d", line)
			for i, old := range d.GitHub {
				if old.Type == f.Type && old.Provider == f.Provider && old.Value == f.Value {
					for _, source := range f.Sources {
						encoded, _ := json.Marshal(source)
						seen := false
						for _, s := range d.GitHub[i].Sources {
							v, _ := json.Marshal(s)
							if string(encoded) == string(v) {
								seen = true
								break
							}
						}
						if !seen {
							d.GitHub[i].Sources = append(d.GitHub[i].Sources, source)
						}
					}
					return
				}
			}
			d.GitHub = append(d.GitHub, f)
		})
		return append(warnings, issues...)
	}}
}
