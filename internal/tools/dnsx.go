package tools

import (
	"discoveryx/internal/model"
	"discoveryx/internal/parser"
	"io"
)

func DNSX() Tool {
	return Adapter{name: "dnsx", inputs: []string{"hosts"}, build: func(in Input) ([]Invocation, error) {
		p := raw(in, "dnsx")
		a, e := configArgs(in, "dnsx")
		if e != nil {
			return nil, e
		}
		l := in.Config.Limits["dnsx"]
		a = append(a, "-l", in.HostsFile, "-a", "-aaaa", "-cname", "-j", "-silent", "-rl", num(l.Rate), "-t", num(l.Concurrency), "-retry", "2", "-o", p)
		return []Invocation{{a, p}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		return parser.JSONL(r, func(m map[string]any, _ int) {
			h := parser.Str(m, "host")
			if !in.Scope.Contains(h) {
				d.External = append(d.External, h)
				return
			}
			ips := parser.Unique(append(parser.Strings(m["a"]), parser.Strings(m["aaaa"])...))
			if len(ips) == 0 {
				return
			}
			for _, c := range parser.Strings(m["cname"]) {
				if !in.Scope.Contains(c) {
					d.External = append(d.External, c)
				}
			}
			for i := range d.Assets {
				if d.Assets[i].Hostname == h {
					d.Assets[i].IPs = ips
					d.Assets[i].DNS = m
					d.Assets[i].Resolved = true
					return
				}
			}
			d.Assets = append(d.Assets, model.Asset{Hostname: h, IPs: ips, DNS: m, Resolved: true, DiscoveredBy: "dnsx"})
		})
	}}
}
