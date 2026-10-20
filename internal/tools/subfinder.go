package tools

import (
	"bufio"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/scope"
	"io"
	"strings"
)

func Subfinder() Tool {
	return Adapter{name: "subfinder", build: func(in Input) ([]Invocation, error) {
		p := raw(in, "subfinder")
		p = strings.TrimSuffix(p, ".jsonl") + ".txt"
		args, e := configArgs(in, "subfinder")
		if e != nil {
			return nil, e
		}
		h, _ := scope.Host(in.Config.Target)
		args = append(args, "-d", h, "-silent", "-rl", num(in.Config.Limits["subfinder"].Rate), "-o", p)
		return []Invocation{{args, p}}, nil
	}, parse: func(r io.Reader, d *model.Data, in Input) []string {
		s := bufio.NewScanner(r)
		s.Buffer(make([]byte, 4096), 1<<20)
		seen := map[string]bool{}
		for _, a := range d.Assets {
			seen[a.Hostname] = true
		}
		for s.Scan() {
			h, e := scope.Host(strings.TrimSpace(s.Text()))
			if e != nil {
				continue
			}
			if !in.Scope.Contains(h) {
				d.External = append(d.External, h)
				continue
			}
			if !seen[h] {
				seen[h] = true
				d.Assets = append(d.Assets, model.Asset{Hostname: h, DiscoveredBy: "subfinder"})
			}
		}
		if e := s.Err(); e != nil {
			return []string{e.Error()}
		}
		return nil
	}}
}
