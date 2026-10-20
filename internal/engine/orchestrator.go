package engine

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/naroSEC/DiscoveryX/internal/config"
	"github.com/naroSEC/DiscoveryX/internal/model"
	"github.com/naroSEC/DiscoveryX/internal/parser"
	"github.com/naroSEC/DiscoveryX/internal/reporting"
	"github.com/naroSEC/DiscoveryX/internal/runner"
	"github.com/naroSEC/DiscoveryX/internal/scope"
	"github.com/naroSEC/DiscoveryX/internal/storage"
	"github.com/naroSEC/DiscoveryX/internal/tools"
	"github.com/naroSEC/DiscoveryX/internal/updater"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Installer interface {
	Ensure(context.Context, string, bool) (updater.Installation, error)
}
type Engine struct {
	ResolveHost func(context.Context, string) ([]string, error)
	Config      config.Config
	Dir         string
	Meta        Metadata
	State       State
	Registry    map[string]tools.Tool
	Executor    runner.Executor
	Manager     Installer
	Out         io.Writer
	log, errors io.Writer
}

func New(c config.Config, out io.Writer) *Engine {
	return &Engine{Config: c, Out: out, Registry: tools.Registry(), Executor: runner.OSExecutor{}, Manager: updater.New(c.ToolsDir, out), ResolveHost: net.DefaultResolver.LookupHost}
}
func (e *Engine) Initialize() error {
	if err := e.Config.Validate(); err != nil {
		return err
	}
	if err := e.Config.Absolute(); err != nil {
		return err
	}
	v, err := scope.New(e.Config.Target, e.Config.Excluded)
	if err != nil {
		return err
	}
	host, _ := scope.Host(e.Config.Target)
	e.Dir = filepath.Join(e.Config.Output, host+"_"+time.Now().Format("20060102_150405.000000000"))
	if err = os.MkdirAll(e.Dir, 0700); err != nil {
		return err
	}
	for _, d := range []string{"inputs", "raw", "normalized", "logs", "sandbox"} {
		if err = os.MkdirAll(filepath.Join(e.Dir, d), 0700); err != nil {
			return err
		}
	}
	e.Meta = Metadata{Schema: 1, Started: time.Now().UTC(), Config: e.Config, Scope: v, Tools: map[string]updater.Installation{}, State: "Pending"}
	e.Meta.Platform = runtime.GOOS
	for _, s := range Plan(e.Config) {
		e.State.Stages = append(e.State.Stages, model.StageState{Name: s.Name, Status: "Pending"})
	}
	e.State.Data.Assets = []model.Asset{{Hostname: host, DiscoveredBy: "input"}}
	if err = storage.JSON(filepath.Join(e.Dir, "inputs", "scope.json"), v); err != nil {
		return err
	}
	return e.save()
}
func Resume(dir string, out io.Writer) (*Engine, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	var meta Metadata
	var state State
	if err = storage.ReadJSON(filepath.Join(abs, "metadata.json"), &meta); err != nil {
		return nil, err
	}
	if err = storage.ReadJSON(filepath.Join(abs, "status.json"), &state); err != nil {
		return nil, err
	}
	if meta.Schema != 1 {
		return nil, fmt.Errorf("incompatible run schema")
	}
	if err = meta.Config.Validate(); err != nil {
		return nil, err
	}
	v, err := scope.New(meta.Config.Target, meta.Config.Excluded)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(v) != fmt.Sprint(meta.Scope) {
		return nil, fmt.Errorf("scope metadata mismatch")
	}
	plan := Plan(meta.Config)
	if len(plan) != len(state.Stages) {
		return nil, fmt.Errorf("stage plan changed; start a new run")
	}
	for i, s := range plan {
		if s.Name != state.Stages[i].Name {
			return nil, fmt.Errorf("stage plan mismatch")
		}
	}
	e := New(meta.Config, out)
	e.Dir = abs
	e.Meta = meta
	e.State = state
	return e, nil
}
func (e *Engine) save() error {
	e.Meta.Updated = time.Now().UTC()
	if err := storage.JSON(filepath.Join(e.Dir, "metadata.json"), e.Meta); err != nil {
		return err
	}
	return storage.JSON(filepath.Join(e.Dir, "status.json"), e.State)
}
func (e *Engine) Run(ctx context.Context, resume bool) (err error) {
	unlock, err := storage.Lock(e.Dir)
	if err != nil {
		return err
	}
	defer unlock()
	unlockTools, err := storage.Lock(e.Config.ToolsDir)
	if err != nil {
		return err
	}
	defer unlockTools()
	unlockTemplates, err := storage.Lock(e.Config.TemplatesDir + ".management")
	if err != nil {
		return err
	}
	defer unlockTemplates()
	log, err := os.OpenFile(filepath.Join(e.Dir, "logs", "execution.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	errs, err := os.OpenFile(filepath.Join(e.Dir, "logs", "errors.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer errs.Close()
	e.log = io.MultiWriter(e.Out, log)
	if e.Config.LogLevel == "error" {
		e.log = log
	}
	e.errors = io.MultiWriter(e.Out, log, errs)
	if resume {
		for i := range e.State.Commands {
			if e.State.Commands[i].Status == "Running" {
				e.State.Commands[i].Status = "Interrupted"
				e.State.Commands[i].Error = "Previous process ended before completion was recorded"
			}
		}
		for path, hash := range e.Meta.Resources {
			now, he := storage.Hash(path)
			if he != nil || now != hash {
				return fmt.Errorf("resume incompatible wordlist/rules resource: %s", path)
			}
		}
		manifest, err := updater.ReadManifest(e.Config.ToolsDir)
		if err != nil {
			return err
		}
		for n, old := range e.Meta.Tools {
			now, ok := manifest[n]
			if !ok || old.SHA256 != now.SHA256 || old.Version != now.Version {
				return fmt.Errorf("resume incompatible tool %s; restore original version", n)
			}
			h, err := storage.Hash(old.Path)
			if err != nil || h != old.SHA256 {
				return fmt.Errorf("resume binary integrity mismatch %s", n)
			}
		}
		if e.Meta.TemplateHash != "" {
			hash, _, err := tools.Tree(e.Config.TemplatesDir)
			if err != nil || hash != e.Meta.TemplateHash {
				return fmt.Errorf("resume incompatible template tree")
			}
		}
	}
	remaining := e.Config.MaxDuration - e.State.ActiveDuration
	if remaining <= 0 {
		return fmt.Errorf("maximum execution duration exhausted")
	}
	ctx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	began := time.Now()
	baseDuration := e.State.ActiveDuration
	e.Meta.State = "Running"
	if err = e.save(); err != nil {
		return err
	}
	defer func() {
		e.State.ActiveDuration = baseDuration + time.Since(began)
		if ctx.Err() != nil {
			e.Meta.State = "Interrupted"
		} else if err != nil {
			e.Meta.State = "Failed"
		} else {
			e.Meta.State = "Completed"
			for _, s := range e.State.Stages {
				if s.Status == "Failed" || s.Status == "Skipped" {
					e.Meta.State = "Partial"
				}
			}
		}
		if se := e.save(); se != nil {
			err = errors.Join(err, se)
		}
		if se := e.writeResults(); se != nil {
			err = errors.Join(err, se)
		}
	}()
	host, _ := scope.Host(e.Config.Target)
	e.State.Data = model.Data{Assets: []model.Asset{{Hostname: host, DiscoveredBy: "input"}}}
	e.State.ScannedIPs = nil
	if e.Config.WebOnly {
		u, err := WebTarget(ctx, e.Config.Target, e.Config.HTTPTimeout, e.Meta.Scope)
		if err != nil {
			return err
		}
		h, _ := scope.Host(u)
		e.State.Data.WebServices = []model.WebService{{URL: u, Hostname: h, Tool: "input"}}
		ep, _ := parser.Endpoint(u, "", "", "input", true, false)
		e.State.Data.Endpoints = []model.Endpoint{ep}
	}
	reuse := resume
	plan := Plan(e.Config)
	for i, stage := range plan {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		st := &e.State.Stages[i]
		if reuse && st.Status == "Completed" {
			cp := filepath.Join("normalized", "checkpoint_"+stage.Name+".json")
			if e.verify(st.Files) {
				var snapshot Checkpoint
				if ce := storage.ReadJSON(filepath.Join(e.Dir, cp), &snapshot); ce == nil {
					e.State.Data = snapshot.Data
					e.State.ScannedIPs = snapshot.ScannedIPs
					fmt.Fprintf(e.log, "[%s] Completed (verified checkpoint)\n", stage.Name)
					continue
				}
			}
			fmt.Fprintf(e.errors, "[%s] checkpoint missing/corrupt; rerunning this and subsequent stages\n", stage.Name)
		}
		reuse = false
		*st = model.StageState{Name: stage.Name, Status: "Running", Started: time.Now().UTC()}
		if err = e.save(); err != nil {
			return err
		}
		fmt.Fprintf(e.log, "[%s] Running (%s)\n", stage.Name, stage.Tool)
		start := time.Now()
		count, files, skip, stageErr := e.execute(ctx, stage)
		st.Duration = time.Since(start)
		st.Count = count
		st.Status = "Completed"
		if skip != "" {
			st.Status = "Skipped"
			st.Reason = skip
		}
		if stageErr != nil {
			st.Status = "Failed"
			st.Reason = stageErr.Error()
		}
		if st.Status == "Completed" {
			checkpoint := filepath.Join(e.Dir, "normalized", "checkpoint_"+stage.Name+".json")
			if err = storage.JSON(checkpoint, Checkpoint{e.State.Data, e.State.ScannedIPs}); err != nil {
				return err
			}
			files = append(files, checkpoint)
			st.Files = map[string]string{}
			for _, p := range files {
				h, he := storage.Hash(p)
				if he != nil {
					return he
				}
				rel, re := filepath.Rel(e.Dir, p)
				if re != nil {
					return re
				}
				st.Files[rel] = h
			}
		}
		fmt.Fprintf(e.log, "[%s] %s: %d records, %s %s\n", stage.Name, st.Status, count, st.Duration.Round(time.Millisecond), st.Reason)
		if stageErr != nil || skip != "" {
			fmt.Fprintf(errs, "%s: %s\n", stage.Name, st.Reason)
		}
		e.State.ActiveDuration = baseDuration + time.Since(began)
		if err = e.save(); err != nil {
			return err
		}
		if err = e.writeResults(); err != nil {
			return err
		}
	}
	return nil
}
func (e *Engine) verify(files map[string]string) bool {
	if len(files) == 0 {
		return false
	}
	for rel, expected := range files {
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(filepath.Clean(rel), ".."+string(filepath.Separator)) {
			return false
		}
		actual, err := storage.Hash(filepath.Join(e.Dir, rel))
		if err != nil || actual != expected {
			return false
		}
	}
	return true
}
func (e *Engine) execute(parent context.Context, s Stage) (count int, files []string, skip string, err error) {
	ctx, cancel := context.WithTimeout(parent, e.Config.StageTimeout)
	executor := runner.Recorder{Executor: e.Executor, Stage: s.Name, Tool: s.Tool, Save: e.recordCommand}
	defer cancel()
	t, ok := e.Registry[s.Tool]
	if !ok {
		return 0, nil, "", fmt.Errorf("unregistered tool %s", s.Tool)
	}
	in := tools.Input{Config: e.Config, Scope: e.Meta.Scope, RunDir: e.Dir, Phase: s.Phase, Warn: e.errors}
	in.PortHosts = map[string]string{}
	d := &e.State.Data
	host, _ := scope.Host(e.Config.Target)
	hosts := []string{host}
	if s.Tool == "dnsx" {
		for _, a := range d.Assets {
			hosts = append(hosts, a.Hostname)
		}
	}
	if s.Tool == "httpx" {
		if e.Config.WebOnly {
			hosts = nil
			for _, w := range d.WebServices {
				hosts = append(hosts, w.URL)
			}
		} else {
			for _, a := range d.Assets {
				if a.Resolved {
					hosts = append(hosts, a.Hostname)
				}
			}
		}
	}
	hosts = parser.Unique(hosts)
	hosts = e.limit(hosts)
	if s.Tool == "naabu" {
		var candidates []string
		if s.Phase == "initial" {
			candidates = []string{host}
		} else {
			for _, a := range d.Assets {
				if a.Resolved || a.Hostname == host {
					candidates = append(candidates, a.Hostname)
				}
			}
		}
		var ips []string
		for _, h := range e.limit(parser.Unique(candidates)) {
			if !in.Scope.Contains(h) {
				continue
			}
			addresses, er := e.ResolveHost(ctx, h)
			if er != nil {
				fmt.Fprintf(e.errors, "port DNS %s: %v\n", h, er)
				continue
			}
			for _, ip := range addresses {
				if !contains(e.State.ScannedIPs, ip) {
					ips = append(ips, ip)
					in.PortHosts[ip] = h
				}
			}
		}
		hosts = e.limit(parser.Unique(ips))
		if len(hosts) == 0 {
			return 0, nil, "no new resolved IPs for port scan", nil
		}
	}
	in.Hosts = hosts
	in.HostsFile = filepath.Join(e.Dir, "inputs", s.Name+"-hosts.txt")
	if err = storage.Lines(in.HostsFile, hosts); err != nil {
		return
	}
	files = append(files, in.HostsFile)
	if s.Tool == "dnsx" {
		if err = storage.Lines(filepath.Join(e.Dir, "inputs", "hosts.txt"), hosts); err != nil {
			return
		}
	}
	var urls []string
	for _, w := range d.WebServices {
		if in.Scope.Contains(w.URL) {
			urls = append(urls, w.URL)
			in.Services = append(in.Services, w)
		}
	}
	urls = e.limit(parser.Unique(urls))
	if len(in.Services) > e.Config.MaxTargets {
		in.Services = in.Services[:e.Config.MaxTargets]
	}
	in.URLsFile = filepath.Join(e.Dir, "inputs", "web-urls.txt")
	if err = storage.Lines(in.URLsFile, urls); err != nil {
		return
	}
	if s.Tool == "katana" || s.Tool == "feroxbuster" || s.Tool == "nuclei" {
		if len(urls) == 0 {
			return 0, files, "no live web service URLs", nil
		}
		files = append(files, in.URLsFile)
	}
	if s.Tool == "feroxbuster" {
		if st, er := os.Stat(e.Config.Wordlist); er != nil || st.IsDir() {
			return 0, files, "wordlist missing; provide " + e.Config.Wordlist + " or --wordlist", nil
		}
		if e.Meta.Resources == nil {
			e.Meta.Resources = map[string]string{}
		}
		for _, resource := range []struct {
			source string
			dest   *string
			name   string
		}{{e.Config.Wordlist, &in.Config.Wordlist, "wordlist.txt"}, {e.Config.TechRules, &in.Config.TechRules, "tech-extensions.yaml"}} {
			var b []byte
			var re error
			if resource.name == "tech-extensions.yaml" {
				b, re = config.ReadTechRules(resource.source)
			} else {
				b, re = os.ReadFile(resource.source)
			}
			if re != nil {
				return 0, files, "", re
			}
			if resource.source != config.BuiltinTechRules {
				e.Meta.Resources[resource.source] = fmt.Sprintf("%x", sha256.Sum256(b))
			}
			dest := filepath.Join(e.Dir, "inputs", resource.name)
			if re = storage.Atomic(dest, b); re != nil {
				return 0, files, "", re
			}
			*resource.dest = dest
			files = append(files, dest)
		}
	}
	installation, er := e.Manager.Ensure(ctx, s.Tool, false)
	if er != nil {
		return 0, files, "", er
	}
	e.Meta.Tools[s.Tool] = installation
	if err = e.save(); err != nil {
		return
	}
	if err = t.Validate(ctx, executor, installation.Path); err != nil {
		return
	}
	helpFlag := "-h"
	if s.Tool == "feroxbuster" {
		helpFlag = "--help"
	}
	in.Help, err = runner.Probe(ctx, executor, installation.Path, helpFlag)
	if err != nil {
		return
	}
	if s.Tool == "nuclei" {
		if e.Config.TemplateAutoInstall {
			fmt.Fprintln(e.log, "nuclei templates: checking local tree; missing templates will be installed")
			installed, installErr := tools.InstallMissingTemplates(ctx, executor, installation.Path, e.Config.TemplatesDir)
			if installErr != nil {
				return 0, files, "", installErr
			}
			if installed {
				fmt.Fprintf(e.log, "nuclei templates: initial installation completed in %s\n", e.Config.TemplatesDir)
			}
		}
		var report tools.FilterReport
		in.TemplatePaths, report, err = tools.SelectedTemplates(in)
		reportPath := filepath.Join(e.Dir, "inputs", "template-filters.json")
		if se := storage.JSON(reportPath, report); se != nil {
			err = se
			return
		}
		files = append(files, reportPath)
		if err != nil {
			return 0, files, err.Error(), nil
		}
		for tag, n := range report.Tags {
			fmt.Fprintf(e.log, "nuclei tag %s: %d installed templates\n", tag, n)
		}
		if len(in.TemplatePaths) == 0 {
			return 0, files, "no eligible templates matched; vulnerability checks not performed", nil
		}
		e.Meta.TemplateHash = report.TreeSHA256
		var ts tools.TemplateState
		_ = storage.ReadJSON(filepath.Join(e.Config.TemplatesDir, "discoveryx-templates.json"), &ts)
		in.TemplateVersion = ts.VersionOutput
		if in.TemplateVersion == "" {
			in.TemplateVersion = "tree-sha256:" + report.TreeSHA256
		}
		e.Meta.TemplateVersion = in.TemplateVersion
		files = append(files, in.TemplatePaths...)
	}

	calls, er := t.BuildArgs(in)
	if er != nil {
		return 0, files, "", er
	}
	if len(calls) == 0 {
		return 0, files, "no targets", nil
	}
	if s.Tool == "nuclei" {
		help, pe := runner.Probe(ctx, executor, installation.Path, "-h")
		if pe != nil {
			return 0, files, "", pe
		}
		for _, flag := range []string{"-tl", "-duc", "-t", "-ud", "-tags", "-s", "-pt", "-dut", "-etags", "-ni"} {
			if !tools.Supports(help, flag) {
				return 0, files, "", fmt.Errorf("nuclei lacks filter validation flag %s", flag)
			}
		}
		list := filepath.Join(e.Dir, "inputs", "selected-templates.txt")
		args := []string{"-tl", "-duc", "-config", filepath.Join(e.Dir, "inputs", "nuclei-config.yaml"), "-t", list, "-ud", e.Config.TemplatesDir, "-tags", e.Config.Tags, "-s", e.Config.Severity, "-pt", "http", "-dut", "-etags", "dos,fuzz,fuzzing,headless,code,oast,unsafe", "-ni"}
		selection, ce := runner.Capture(ctx, executor, installation.Path, args...)
		if se := storage.Atomic(filepath.Join(e.Dir, "raw", "nuclei-template-list.log"), []byte(selection)); se != nil {
			return 0, files, "", se
		}
		if ce != nil {
			return 0, files, "", fmt.Errorf("template engine validation: %w", ce)
		}
		selected := 0
		for _, line := range strings.Split(selection, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, ".yaml") && !strings.HasPrefix(line, "[") {
				selected++
			}
		}
		var report tools.FilterReport
		rp := filepath.Join(e.Dir, "inputs", "template-filters.json")
		if ce = storage.ReadJSON(rp, &report); ce != nil {
			return 0, files, "", ce
		}
		report.EngineSelected = selected
		if ce = storage.JSON(rp, report); ce != nil {
			return 0, files, "", ce
		}
		if selected == 0 {
			return 0, files, "Nuclei selected zero valid/signed templates; checks not performed", nil
		}
	}
	before := dataCount(*d)
	beforeHTTPX := 0
	if s.Tool == "httpx" {
		for _, w := range d.WebServices {
			if w.Tool == "httpx" {
				beforeHTTPX++
			}
		}
	}
	var runErrors []error
	for _, call := range calls {
		if ctx.Err() != nil {
			return count, files, "", ctx.Err()
		}
		if er = e.archivePrevious(call.Output); er != nil {
			return count, files, "", er
		}
		if e.Config.LogLevel == "debug" {
			fmt.Fprintf(e.log, "command: %q %q\n", installation.Path, call.Args)
		}
		runErr := t.Run(ctx, executor, installation.Path, call, in)
		if runErr != nil {
			runErrors = append(runErrors, runErr)
		}
		f, fe := os.Open(call.Output)
		if fe == nil {
			warnings := t.Parse(f, d, in)
			f.Close()
			files = append(files, call.Output)
			for _, warning := range warnings {
				fmt.Fprintf(e.errors, "%s parse: %s\n", s.Name, warning)
			}
			if len(warnings) > 0 {
				runErrors = append(runErrors, fmt.Errorf("%s malformed/truncated output (%d warnings); partial records retained", s.Tool, len(warnings)))
			}
		} else {
			runErrors = append(runErrors, fmt.Errorf("required raw output missing: %w", fe))
		}
		if runErr != nil {
			fmt.Fprintf(e.errors, "%s: %v (see %s.stderr.log)\n", s.Name, runErr, call.Output)
		}
	}
	if s.Tool == "nuclei" {
		var logs strings.Builder
		for _, call := range calls {
			for _, suffix := range []string{".stderr.log", ".stdout.log"} {
				b, _ := os.ReadFile(call.Output + suffix)
				logs.Write(b)
			}
		}
		loaded, known := tools.LoadedTemplates(logs.String())
		var report tools.FilterReport
		rp := filepath.Join(e.Dir, "inputs", "template-filters.json")
		if re := storage.ReadJSON(rp, &report); re != nil {
			return 0, files, "", re
		}
		report.EngineLoaded = loaded
		if re := storage.JSON(rp, report); re != nil {
			return 0, files, "", re
		}
		if !known || loaded == 0 {
			runErrors = append(runErrors, fmt.Errorf("Nuclei did not confirm any loaded signed templates; vulnerability checks cannot be marked completed"))
		}
	}
	count = dataCount(*d) - before
	if s.Tool == "httpx" {
		count = -beforeHTTPX
		for _, w := range d.WebServices {
			if w.Tool == "httpx" {
				count++
			}
		}
	}
	err = errors.Join(runErrors...)
	if s.Tool == "naabu" && err == nil {
		e.State.ScannedIPs = parser.Unique(append(e.State.ScannedIPs, hosts...))
	}
	if s.Tool == "dnsx" {
		d.InvalidHosts = nil
		for _, a := range d.Assets {
			if !a.Resolved {
				d.InvalidHosts = append(d.InvalidHosts, a.Hostname)
			}
		}
	}
	if len(d.Assets) > e.Config.MaxTargets {
		fmt.Fprintf(e.errors, "target count capped at %d for downstream stages\n", e.Config.MaxTargets)
	}
	return
}
func (e *Engine) archivePrevious(path string) error {
	changed := false
	for _, p := range []string{path, path + ".stdout.log", path + ".stderr.log"} {
		if _, err := os.Stat(p); err == nil {
			backup := p + ".previous-" + time.Now().Format("20060102150405.000000000")
			if err = os.Rename(p, backup); err != nil {
				return err
			}
			oldRef, _ := filepath.Rel(e.Dir, p)
			newRef, _ := filepath.Rel(e.Dir, backup)
			for i := range e.State.Commands {
				for _, ref := range []*string{&e.State.Commands[i].Stdout, &e.State.Commands[i].Stderr} {
					if *ref == filepath.ToSlash(oldRef) {
						*ref = filepath.ToSlash(newRef)
						changed = true
					}
				}
			}
		}
	}
	if changed {
		if err := storage.JSON(filepath.Join(e.Dir, "logs", "commands.json"), e.State.Commands); err != nil {
			return err
		}
		return e.save()
	}
	return nil
}
func dataCount(d model.Data) int {
	return len(d.Assets) + len(d.Ports) + len(d.WebServices) + len(d.Endpoints) + len(d.Vulnerabilities)
}
func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
func (e *Engine) limit(ss []string) []string {
	if len(ss) > e.Config.MaxTargets {
		fmt.Fprintf(e.errors, "target list truncated: %d -> %d\n", len(ss), e.Config.MaxTargets)
		return ss[:e.Config.MaxTargets]
	}
	return ss
}
func WebTarget(ctx context.Context, target string, timeout time.Duration, v scope.Validator) (string, error) {
	if strings.Contains(target, "://") {
		return scope.URL(target)
	}
	host, err := scope.Host(target)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	return probeWeb(ctx, []string{"https://" + host, "http://" + host}, client, v)
}
func probeWeb(ctx context.Context, targets []string, client *http.Client, v scope.Validator) (string, error) {
	for _, u := range targets {
		req, err := http.NewRequestWithContext(ctx, "HEAD", u, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if v.Contains(u) {
				return scope.URL(u)
			}
		}
	}
	return "", fmt.Errorf("web-only preflight: HTTPS and HTTP unavailable")
}
func (e *Engine) writeResults() error {
	d := &e.State.Data
	var urls []string
	for _, w := range d.WebServices {
		if e.Meta.Scope.Contains(w.URL) {
			urls = append(urls, w.URL)
		}
	}
	urls = parser.Unique(urls)
	if len(urls) > e.Config.MaxTargets {
		urls = urls[:e.Config.MaxTargets]
	}
	if er := storage.Lines(filepath.Join(e.Dir, "inputs", "web-urls.txt"), urls); er != nil {
		return er
	}
	d.External = parser.Unique(d.External)
	items := []struct {
		name  string
		value any
	}{{"assets", d.Assets}, {"ports", d.Ports}, {"webservices", d.WebServices}, {"urls", d.Endpoints}, {"vulnerabilities", d.Vulnerabilities}, {"out-of-scope", d.External}, {"invalid-hosts", d.InvalidHosts}}
	for _, it := range items {
		if er := storage.JSON(filepath.Join(e.Dir, "normalized", it.name+".json"), it.value); er != nil {
			return er
		}
	}
	s := e.Summary()
	if err := storage.JSON(filepath.Join(e.Dir, "summary.json"), s); err != nil {
		return err
	}
	return reporting.Write(e.Dir, reporting.Snapshot{Target: e.Config.Target, State: e.Meta.State, Started: e.Meta.Started, Updated: e.Meta.Updated, Duration: e.State.ActiveDuration, Output: e.Dir, Platform: e.Meta.Platform, Invocation: e.Meta.Command, Stages: e.State.Stages, Data: e.State.Data, Commands: e.State.Commands, Tools: e.Meta.Tools})
}
func (e *Engine) Summary() Summary {
	s := Summary{Target: e.Config.Target, Duration: e.State.ActiveDuration, WebServices: len(e.State.Data.WebServices), Ports: len(e.State.Data.Ports), Findings: len(e.State.Data.Vulnerabilities), Stages: e.State.Stages, Output: e.Dir, State: e.Meta.State}
	host, _ := scope.Host(e.Config.Target)
	for _, a := range e.State.Data.Assets {
		if a.Hostname != host {
			s.Subdomains++
		}
		if a.Resolved {
			s.Resolved++
		}
	}
	for _, ep := range e.State.Data.Endpoints {
		if ep.Directory {
			s.Directories++
		}
		if ep.Crawled {
			s.URLs++
		}
	}
	for _, v := range e.State.Data.Vulnerabilities {
		if v.DuplicateOf == "" {
			s.UniqueFindings++
		}
	}
	return s
}
func (e *Engine) PrintSummary() {
	s := e.Summary()
	if _, err := os.Stat(filepath.Join(e.Dir, "report.html")); err == nil {
		fmt.Fprintf(e.Out, "HTML report: %s\n", filepath.Join(e.Dir, "report.html"))
	}
	fmt.Fprintf(e.Out, "\nTarget: %s\nState: %s | Duration: %s\nSubdomains: %d | Resolved: %d | Web services: %d\nPorts: %d | Crawled URLs: %d | Directories: %d\nFindings: %d events / %d unique (review required)\nOutput: %s\n", s.Target, s.State, s.Duration.Round(time.Millisecond), s.Subdomains, s.Resolved, s.WebServices, s.Ports, s.URLs, s.Directories, s.Findings, s.UniqueFindings, s.Output)
	for _, st := range s.Stages {
		if st.Status == "Failed" || st.Status == "Skipped" {
			fmt.Fprintf(e.Out, "%s %s: %s\n", st.Name, st.Status, st.Reason)
		}
	}
}
