package tools

import (
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/parser"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type TechRule struct {
	Match      []string `yaml:"match"`
	Extensions []string `yaml:"extensions"`
}
type TechRules struct {
	Base  []string   `yaml:"base"`
	Rules []TechRule `yaml:"rules"`
}

func Extensions(tech, extra []string, r TechRules) ([]string, error) {
	all := append(append([]string{}, r.Base...), extra...)
	for _, rule := range r.Rules {
		matched := false
		for _, pattern := range rule.Match {
			re, e := regexp.Compile("(?i)" + pattern)
			if e != nil {
				return nil, e
			}
			for _, t := range tech {
				if re.MatchString(t) {
					matched = true
				}
			}
		}
		if matched {
			all = append(all, rule.Extensions...)
		}
	}
	out := []string{}
	for _, e := range all {
		e = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(e)), ".")
		if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(e) {
			return nil, fmt.Errorf("invalid extension %q", e)
		}
		out = append(out, e)
	}
	out = parser.Unique(out)
	sort.Strings(out)
	return out, nil
}
func Feroxbuster() Tool {
	return Adapter{name: "feroxbuster", inputs: []string{"web_urls", "wordlist"}, build: func(in Input) ([]Invocation, error) {
		if st, e := os.Stat(in.Config.Wordlist); e != nil || st.IsDir() {
			return nil, fmt.Errorf("wordlist missing: provide %s or --wordlist; DiscoveryX never downloads/creates a wordlist", in.Config.Wordlist)
		}
		b, e := config.ReadTechRules(in.Config.TechRules)
		if e != nil {
			return nil, e
		}
		var r TechRules
		if e = yaml.Unmarshal(b, &r); e != nil {
			return nil, e
		}
		if len(r.Base) == 0 {
			return nil, fmt.Errorf("technology rules need base extensions")
		}
		var calls []Invocation
		l := in.Config.Limits["feroxbuster"]
		for i, w := range in.Services {
			ext, e := Extensions(w.Technologies, in.Config.Extensions, r)
			if e != nil {
				return nil, e
			}
			p := raw(in, fmt.Sprintf("feroxbuster_%04d", i))
			work := filepath.Join(in.RunDir, "sandbox", "feroxbuster")
			if e = os.MkdirAll(work, 0700); e != nil {
				return nil, e
			}
			if e = storage.Atomic(filepath.Join(work, "ferox-config.toml"), []byte("redirects = false\n")); e != nil {
				return nil, e
			}
			a := []string{"-u", w.URL, "-x", strings.Join(ext, ","), "--depth", num(in.Config.DirectoryDepth), "-w", in.Config.Wordlist, "-s", "200", "301", "302", "--rate-limit", num(l.Rate), "-t", num(l.Concurrency), "--scan-limit", "1", "--timeout", timeout(in), "--no-state", "--json", "-o", p}
			if re := in.Scope.ExcludeRegex(); re != "" {
				a = append(a, "--dont-scan", re)
			}
			a = append(a, "--dont-extract-links")
			calls = append(calls, Invocation{a, p})
		}
		return calls, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		return parser.JSONL(r, func(m map[string]any, _ int) {
			if t := parser.Str(m, "type"); t != "response" && t != "" {
				return
			}
			s := parser.Str(m, "url")
			if !in.Scope.Contains(s) {
				if s != "" {
					d.External = append(d.External, s)
				}
				return
			}
			ep, e := parser.Endpoint(s, "", parser.Str(m, "method"), "feroxbuster", false, true)
			if e != nil {
				return
			}
			parser.AddEndpoint(d, ep)
		})
	}}
}
