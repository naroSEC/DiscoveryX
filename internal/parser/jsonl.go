package parser

import (
	"bufio"
	"discoveryx/internal/model"
	"discoveryx/internal/scope"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

func JSONL(r io.Reader, fn func(map[string]any, int)) []string {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 65536), 16<<20)
	line := 0
	var warnings []string
	for s.Scan() {
		line++
		b := s.Bytes()
		if len(strings.TrimSpace(string(b))) == 0 {
			continue
		}
		var m map[string]any
		if e := json.Unmarshal(b, &m); e != nil || m == nil {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid JSON object", line))
			continue
		}
		fn(m, line)
	}
	if e := s.Err(); e != nil {
		warnings = append(warnings, e.Error())
	}
	return warnings
}
func Str(m map[string]any, k string) string { v, _ := m[k].(string); return v }
func Int(m map[string]any, k string) int    { v, _ := m[k].(float64); return int(v) }
func Map(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	if v == nil {
		return map[string]any{}
	}
	return v
}
func Strings(v any) []string {
	var out []string
	switch x := v.(type) {
	case string:
		if x != "" {
			out = append(out, x)
		}
	case []any:
		for _, s := range x {
			if v, ok := s.(string); ok {
				out = append(out, v)
			}
		}
	}
	return out
}
func Unique(ss []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
func Endpoint(raw, source, method, tool string, initial, directory bool) (model.Endpoint, error) {
	s, e := scope.URL(raw)
	if e != nil {
		return model.Endpoint{}, e
	}
	u, _ := url.Parse(s)
	return model.Endpoint{URL: s, Hostname: u.Hostname(), Path: u.Path, Query: u.Query(), Source: source, Method: method, Tool: tool, SourceTools: []string{tool}, Crawled: tool == "katana", Initial: initial, Directory: directory}, nil
}
func AddEndpoint(d *model.Data, ep model.Endpoint) {
	for i := range d.Endpoints {
		old := &d.Endpoints[i]
		if old.URL == ep.URL {
			old.Initial = old.Initial || ep.Initial
			old.Crawled = old.Crawled || ep.Crawled
			old.Directory = old.Directory || ep.Directory
			old.SourceTools = Unique(append(old.SourceTools, ep.SourceTools...))
			if ep.Source != "" {
				old.Source = ep.Source
			}
			if ep.Method != "" {
				old.Method = ep.Method
			}
			return
		}
	}
	d.Endpoints = append(d.Endpoints, ep)
}
