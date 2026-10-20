package tools

import (
	"discoveryx/internal/model"
	"discoveryx/internal/parser"
	"discoveryx/internal/scope"
	"io"
)

func HTTPX() Tool {
	return Adapter{name: "httpx", inputs: []string{"hosts"}, build: func(in Input) ([]Invocation, error) {
		p := raw(in, "httpx")
		a, e := configArgs(in, "httpx")
		if e != nil {
			return nil, e
		}
		l := in.Config.Limits["httpx"]
		a = append(a, "-l", in.HostsFile, "-silent", "-sc", "-title", "-td", "-ip", "-cdn", "-server", "-j", "-rl", num(l.Rate), "-t", num(l.Concurrency), "-timeout", timeout(in), "-o", p)
		return []Invocation{{a, p}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		return parser.JSONL(r, func(m map[string]any, _ int) {
			s, e := scope.URL(parser.Str(m, "url"))
			if e != nil {
				return
			}
			if !in.Scope.Contains(s) {
				d.External = append(d.External, s)
				return
			}
			h, _ := scope.Host(s)
			for _, w := range d.WebServices {
				if w.URL == s {
					return
				}
			}
			if parser.Int(m, "status_code") == 0 {
				return
			}
			ip := parser.Str(m, "host")
			if ip == "" {
				ip = parser.Str(m, "ip")
			}
			cdn := parser.Str(m, "cdn_name")
			if w := parser.Str(m, "cdn_type"); w != "" {
				cdn += " " + w
			}
			d.WebServices = append(d.WebServices, model.WebService{URL: s, Hostname: h, IP: ip, Status: parser.Int(m, "status_code"), Title: parser.Str(m, "title"), Technologies: parser.Strings(m["tech"]), Server: parser.Str(m, "webserver"), CDN: cdn, Tool: "httpx"})
			for i := range d.Assets {
				if d.Assets[i].Hostname == h {
					d.Assets[i].WebServices = parser.Unique(append(d.Assets[i].WebServices, s))
				}
			}
			ep, e := parser.Endpoint(s, "", "", "httpx", true, false)
			if e == nil {
				parser.AddEndpoint(d, ep)
			}
		})
	}}
}
