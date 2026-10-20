package tools

import (
	"context"
	"crypto/sha256"
	"discoveryx/internal/runner"
	"discoveryx/internal/storage"
	"encoding/hex"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type TemplateState struct {
	VersionOutput string    `json:"version_output"`
	TreeSHA256    string    `json:"tree_sha256"`
	Updated       time.Time `json:"updated"`
	Files         int       `json:"files"`
	Status        string    `json:"status"`
}
type FilterReport struct {
	EngineLoaded   int            `json:"engine_loaded_signed_templates"`
	EligibleByTag  map[string]int `json:"eligible_by_tag"`
	Tags           map[string]int `json:"available_tag_counts"`
	Eligible       int            `json:"eligible_http_templates"`
	Rejected       int            `json:"rejected"`
	EngineSelected int            `json:"engine_selected"`
	TreeSHA256     string         `json:"tree_sha256"`
	Policy         string         `json:"policy"`
}

func Tree(dir string) (string, int, error) {
	h := sha256.New()
	count := 0
	e := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in templates: %s", p)
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".yaml") {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(dir, p)
		fmt.Fprintf(h, "%s\x00%x\n", filepath.ToSlash(rel), sha256.Sum256(b))
		count++
		return nil
	})
	if e != nil {
		return "", 0, e
	}
	return hex.EncodeToString(h.Sum(nil)), count, nil
}
func UpdateTemplates(ctx context.Context, e runner.Executor, exe, dir string) (TemplateState, error) {
	// Update into a staging directory; keep the previous template tree on failure.
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return TemplateState{}, err
	}
	stage, err := os.MkdirTemp(parent, "templates-candidate-")
	if err != nil {
		return TemplateState{}, err
	}
	defer os.RemoveAll(stage)
	help, err := runner.Probe(ctx, e, exe, "-h")
	if err != nil {
		return TemplateState{}, err
	}
	for _, flag := range []string{"-ut", "-ud", "-tv"} {
		if !Supports(help, flag) {
			return TemplateState{}, fmt.Errorf("nuclei lacks %s", flag)
		}
	}
	output, err := runner.Capture(ctx, e, exe, "-ut", "-ud", stage)
	if err != nil {
		return TemplateState{}, fmt.Errorf("template update failed; old tree retained: %w: %s", err, output)
	}
	hash, count, err := Tree(stage)
	if err != nil || count == 0 {
		return TemplateState{}, fmt.Errorf("template update produced no usable tree: %v", err)
	}
	version, err := runner.Capture(ctx, e, exe, "-tv", "-ud", stage, "-duc")
	if err != nil {
		return TemplateState{}, err
	}
	state := TemplateState{VersionOutput: strings.TrimSpace(version), TreeSHA256: hash, Files: count, Updated: time.Now().UTC(), Status: "updated"}
	if err = storage.JSON(filepath.Join(stage, "discoveryx-templates.json"), state); err != nil {
		return state, err
	}
	backup := dir + ".backup-" + time.Now().Format("20060102150405.000000000")
	hadOld := false
	if _, err = os.Stat(dir); err == nil {
		if err = os.Rename(dir, backup); err != nil {
			return state, err
		}
		hadOld = true
	}
	if err = os.Rename(stage, dir); err != nil {
		if hadOld {
			if rollback := os.Rename(backup, dir); rollback != nil {
				return state, fmt.Errorf("template commit: %v; rollback: %v; preserved tree: %s", err, rollback, backup)
			}
		}
		return state, err
	}
	return state, nil
}
func tagSet(v any) []string {
	switch s := v.(type) {
	case string:
		return strings.Split(strings.ToLower(s), ",")
	case []any:
		var out []string
		for _, x := range s {
			out = append(out, tagSet(x)...)
		}
		return out
	}
	return nil
}
func SelectedTemplates(in Input) ([]string, FilterReport, error) {
	report := FilterReport{Tags: map[string]int{}, EligibleByTag: map[string]int{}, Policy: "signed HTTP GET/HEAD, fixed BaseURL/RootURL paths; no payloads/raw/DSL/redirects/OAST/code; default engine ignore rules retained"}
	for _, t := range strings.Split(in.Config.Tags, ",") {
		report.Tags[strings.TrimSpace(strings.ToLower(t))] = 0
		report.EligibleByTag[strings.TrimSpace(strings.ToLower(t))] = 0
	}
	hash, _, err := Tree(in.Config.TemplatesDir)
	if err != nil {
		return nil, report, fmt.Errorf("templates absent; run discoveryx templates update: %w", err)
	}
	report.TreeSHA256 = hash
	var paths []string
	err = filepath.WalkDir(in.Config.TemplatesDir, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var m map[string]any
		if yaml.Unmarshal(b, &m) != nil {
			report.Rejected++
			return nil
		}
		info, _ := m["info"].(map[string]any)
		tags := tagSet(info["tags"])
		selected := false
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if _, ok := report.Tags[t]; ok {
				report.Tags[t]++
				selected = true
			}
		}
		if !selected {
			return nil
		}
		severity, _ := info["severity"].(string)
		if !csvContains(in.Config.Severity, severity) {
			return nil
		}
		if !safeTemplate(m, string(b), tags) {
			report.Rejected++
			return nil
		}
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if _, ok := report.EligibleByTag[t]; ok {
				report.EligibleByTag[t]++
			}
		}
		rel, _ := filepath.Rel(in.Config.TemplatesDir, p)
		dest := filepath.Join(in.RunDir, "inputs", "templates", rel)
		if e = storage.Atomic(dest, b); e != nil {
			return e
		}
		paths = append(paths, dest)
		return nil
	})
	sort.Strings(paths)
	report.Eligible = len(paths)
	return paths, report, err
}
func csvContains(csv, s string) bool {
	for _, x := range strings.Split(csv, ",") {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}
func safeTemplate(m map[string]any, raw string, tags []string) bool {
	if !regexp.MustCompile(`(?m)^# digest: [0-9a-fA-F]+:[0-9a-fA-F]+\s*$`).MatchString(raw) {
		return false
	} // Only a shape check; nuclei -dut verifies the cryptographic signature.
	for _, t := range tags {
		if csvContains("dos,fuzz,fuzzing,headless,code,oast,unsafe,intrusive,bruteforce", t) {
			return false
		}
	}
	for k := range m {
		if !csvContains("id,info,http", k) {
			return false
		}
	}
	lower := strings.ToLower(raw)
	for _, bad := range []string{"interactsh", "oast.", "unsafe:", "redirects:", "payloads:", "race:", "dsl:", "javascript:", "pre-condition:", "fuzzing:"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	requests, ok := m["http"].([]any)
	if !ok || len(requests) == 0 || len(requests) > 10 {
		return false
	}
	for _, r := range requests {
		req, ok := r.(map[string]any)
		if !ok {
			return false
		}
		for k := range req {
			if !csvContains("method,path,headers,matchers,matchers-condition,extractors,stop-at-first-match", k) {
				return false
			}
		}
		method, _ := req["method"].(string)
		if method != "GET" && method != "HEAD" {
			return false
		}
		paths, ok := req["path"].([]any)
		if !ok || len(paths) == 0 || len(paths) > 20 {
			return false
		}
		for _, p := range paths {
			s, ok := p.(string)
			if !ok {
				return false
			}
			rest := ""
			switch {
			case strings.HasPrefix(s, "{{BaseURL}}/"):
				rest = strings.TrimPrefix(s, "{{BaseURL}}")
			case strings.HasPrefix(s, "{{RootURL}}/"):
				rest = strings.TrimPrefix(s, "{{RootURL}}")
			default:
				return false
			}
			if strings.Contains(rest, "{{") || strings.ContainsAny(rest, "\r\n") {
				return false
			}
		}
		if headers, ok := req["headers"].(map[string]any); ok {
			for k, v := range headers {
				if strings.EqualFold(k, "host") {
					return false
				}
				if strings.Contains(fmt.Sprint(v), "{{") {
					return false
				}
			}
		}
	}
	return true
}
