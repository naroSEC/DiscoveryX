package tools

import (
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/parser"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
)

func Nuclei() Tool {
	return Adapter{name: "nuclei", inputs: []string{"web_urls", "templates"}, build: func(in Input) ([]Invocation, error) {
		if len(in.TemplatePaths) == 0 {
			return nil, fmt.Errorf("no eligible installed HTTP templates; run discoveryx templates update explicitly or inspect template-filters.json")
		}
		list := filepath.Join(in.RunDir, "inputs", "selected-templates.txt")
		if e := storage.Lines(list, in.TemplatePaths); e != nil {
			return nil, e
		}
		p := raw(in, "nuclei")
		a, e := configArgs(in, "nuclei")
		if e != nil {
			return nil, e
		}
		l := in.Config.Limits["nuclei"]
		a = append(a, "-nc")
		a = append(a, "-l", in.URLsFile, "-t", list, "-ud", in.Config.TemplatesDir, "-tags", in.Config.Tags, "-s", in.Config.Severity, "-pt", "http", "-etags", "dos,fuzz,fuzzing,headless,code,oast,unsafe", "-ni", "-dr", "-dut", "-timeout", timeout(in), "-j", "-o", p)
		if l.Rate > 0 {
			a = append(a, "-rl", num(l.Rate))
		}
		if l.Concurrency > 0 {
			a = append(a, "-c", num(l.Concurrency))
		}
		return []Invocation{{a, p}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		return parser.JSONL(r, func(m map[string]any, line int) {
			matched := parser.Str(m, "matched-at")
			if !in.Scope.Contains(matched) {
				d.External = append(d.External, matched)
				return
			}
			info := parser.Map(m, "info")
			classification := parser.Map(info, "classification")
			id := parser.Str(m, "template-id")
			matcher := parser.Str(m, "matcher-name")
			v := model.Vulnerability{TemplateID: id, Name: parser.Str(info, "name"), Severity: parser.Str(info, "severity"), CVE: parser.Strings(classification["cve-id"]), Target: parser.Str(m, "host"), Matched: matched, Matcher: matcher, Evidence: fmt.Sprintf("raw/nuclei.jsonl:%d", line), Time: parser.Str(m, "timestamp"), TemplateVersion: in.TemplateVersion, Tool: "nuclei"}
			for _, old := range d.Vulnerabilities {
				if old.TemplateID == id && old.Matched == matched && old.Matcher == matcher {
					v.DuplicateOf = old.Evidence
					break
				}
			}
			d.Vulnerabilities = append(d.Vulnerabilities, v)
		})
	}}
}

func LoadedTemplates(log string) (int, bool) {
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(log, "")
	m := regexp.MustCompile(`(?i)Templates loaded for current scan:\s*(\d+)`).FindStringSubmatch(plain)
	if len(m) == 2 {
		n, e := strconv.Atoi(m[1])
		return n, e == nil
	}
	return 0, false
}
