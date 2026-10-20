package tools

import (
	"discoveryx/internal/model"
	"discoveryx/internal/parser"
	"fmt"
	"io"
	"regexp"
	"strings"
)

func Katana() Tool {
	return Adapter{name: "katana", inputs: []string{"web_urls"}, build: func(in Input) ([]Invocation, error) {
		p := raw(in, "katana")
		a, e := configArgs(in, "katana")
		if e != nil {
			return nil, e
		}
		l := in.Config.Limits["katana"]
		var hosts []string
		for _, w := range in.Services {
			hosts = append(hosts, regexp.QuoteMeta(w.Hostname))
		}
		if len(hosts) == 0 {
			return nil, fmt.Errorf("no input FQDNs for crawl scope")
		}
		crawlScope := `(?i)^https?://(?:` + strings.Join(parser.Unique(hosts), "|") + `)(?::[0-9]+)?(?:/|$)`
		a = append(a, "-u", in.URLsFile, "-d", num(in.Config.CrawlDepth), "-jc", "-kf", "all", "-fs", "fqdn", "-cs", crawlScope, "-dr", "-do", "-c", num(l.Concurrency), "-p", "1", "-rl", num(l.Rate), "-timeout", timeout(in), "-j", "-o", p)
		if re := in.Scope.ExcludeRegex(); re != "" {
			a = append(a, "-cos", re)
		}
		return []Invocation{{a, p}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		return parser.JSONL(r, func(m map[string]any, _ int) {
			req := parser.Map(m, "request")
			s := parser.Str(req, "endpoint")
			if s == "" {
				s = parser.Str(m, "url")
			}
			if !in.Scope.Contains(s) {
				if s != "" {
					d.External = append(d.External, s)
				}
				return
			}
			ep, e := parser.Endpoint(s, parser.Str(req, "source"), parser.Str(req, "method"), "katana", false, false)
			if e != nil {
				return
			}
			parser.AddEndpoint(d, ep)
		})
	}}
}
